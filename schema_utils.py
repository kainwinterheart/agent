import json


def pad(line: str, level: int) -> str:
    return " " * (level * 2) + line


def schema_to_example(schema: dict) -> str:
    def build_value(node, level = 0):
        node_type = node.get("type")

        if "enum" in node and node["enum"]:
            return node.get("description", "/".join(map(json.dumps, node["enum"])))

        if node_type == "string":
            return json.dumps(node.get('description', 'example string'))

        if node_type == "boolean":
            return node.get('description', 'true/false')

        if node_type == "integer":
            return str(node.get('description', 0))

        if node_type == "number":
            return str(node.get('description', 0.0))

        if node_type == "array":
            items = node.get("items", {})
            return f"[{build_value(items, level)}]"

        if node_type == "object":
            properties = node.get("properties", {})
            lines = ["{"]
            for key in properties:
                lines.append(pad(f"{json.dumps(key)}: {build_value(properties[key], level + 2)},", level + 1))
            lines[-1] = lines[-1].rstrip(",")
            lines.append(pad("}", level))
            return "\n".join(lines)

        return "example"

    return build_value(schema)

