#!/usr/bin/env bash

tmux start-server \; \
     new-session  -c scripts -s "mpss" \;  \
     splitw -h "ls -l" \;  \
     selectp -t 0 "ls -l" \; \
     splitw -v "ls -l" \;  \
     selectp -t 2 \; \
     splitw -v "ls" \; \
     tmux attach-session -d
