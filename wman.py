import os
import threading

import pywatchman

from utils import log


class WatchmanBackgroundWatcher:
    def __init__(self, state_dir):
        self.client = pywatchman.client()
        self.thread = None
        self.stop_event = threading.Event()
        self.state_dir = state_dir

        self.watch_root = None
        self.relative_root = None
        self.subscription_name = "agent-watcher"

        self.last_known = {}
        self.changed_files = {}
        self.lock = threading.Lock()
        self._initialized = False

    @property
    def effective_root(self) -> str:
        effective_root = self.watch_root
        if self.relative_root:
            effective_root = os.path.join(self.watch_root, self.relative_root)
        return effective_root

    # ------------------------------------------------------------
    # 1. START
    # ------------------------------------------------------------
    def start(self, path: str):
        """
        Start watching a directory in the background.
        Returns immediately after Watchman subscription is set up.
        """

        resp = self.client.query("watch-project", path)
        self.watch_root = resp["watch"]
        self.relative_root = resp.get("relative_path", "")

        expr = [
            "allof",
            ["type", "f"],
        ]
        to_exclude = safe_relative(self.effective_root, self.state_dir)
        if to_exclude:
            expr.append(["not", ["match", to_exclude + "/*", "wholename"]])
            expr.append(["not", ["dirname", to_exclude]])
        log("WATCHMAN", repr(expr))
        self.client.query(
            "subscribe",
            self.watch_root,
            self.subscription_name,
            {
                "expression": expr,
                "fields": ["name", "size"],
            },
        )

        self.stop_event.clear()

        self.thread = threading.Thread(target=self._run_loop, daemon=True)
        self.thread.start()

    # ------------------------------------------------------------
    # INTERNAL LOOP
    # ------------------------------------------------------------
    def _run_loop(self):
        """
        Blocking Watchman event loop (runs in background thread)
        """
        while not self.stop_event.is_set():
            try:
                self._it()
            except Exception:
                pass

    def _it(self):
        result = self.client.receive()

        if not isinstance(result, dict):
            return

        if result.get("subscription") != self.subscription_name:
            return

        files = result.get("files", [])
        if not isinstance(files, list):
            return

        # --------------------------------------------------------
        # SKIP INITIAL SNAPSHOT
        # --------------------------------------------------------
        with self.lock:
            if not self._initialized:
                for file in files:
                    self.last_known[file.pop("name")] = file
                self._initialized = True
                return

        # --------------------------------------------------------
        # NORMAL CHANGE EVENTS
        # --------------------------------------------------------
        with self.lock:
            for file in files:
                self.changed_files[file.pop("name")] = file

    # ------------------------------------------------------------
    # 2. GET CHANGES
    # ------------------------------------------------------------
    def flush(self):
        out = {}
        with self.lock:
            for file, data in self.changed_files.items():
                if new_size := int(data.get("size", 0)):
                    prev = self.last_known.get(file, {})
                    if prev_size := int(prev.get("size", 0)):
                        if prev_size != new_size:
                            out[file] = "modified"
                    else:
                        out[file] = "created"
                    self.last_known[file] = data
                else:
                    prev = self.last_known.pop(file, {})
                    if int(prev.get("size", 0)):
                        out[file] = "deleted"
        return out

    def wait(self):
        while True:
            with self.lock:
                if self._initialized:
                    return

    # ------------------------------------------------------------
    # 3. STOP
    # ------------------------------------------------------------
    def stop(self):
        """
        Stop watcher and background thread.
        """
        self.stop_event.set()

        try:
            self.client.query(
                "unsubscribe",
                self.watch_root,
                self.subscription_name,
            )
        except Exception:
            pass

        if self.thread:
            self.thread.join(timeout=2)

        try:
            self.client.close()
        except Exception:
            pass


def safe_relative(watch_root, abs_path):
    try:
        rel = os.path.relpath(os.path.abspath(abs_path), watch_root)
        if rel.startswith(".."):
            return None  # outside watch root
        return rel
    except ValueError:
        return None
