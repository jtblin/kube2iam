#!/bin/bash
echo "Downloading manifest-tool."
wget https://github.com/estesp/manifest-tool/releases/download/v0.7.0/manifest-tool-linux-amd64
mv manifest-tool-linux-amd64 /usr/bin/manifest-tool
chmod +x /usr/bin/manifest-tool
manifest-tool --version
