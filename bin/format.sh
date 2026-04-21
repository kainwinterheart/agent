#!/bin/sh -e

. venv/bin/activate

isort --profile black *.py

autoflake -r --in-place --remove-unused-variables *.py

black *.py
