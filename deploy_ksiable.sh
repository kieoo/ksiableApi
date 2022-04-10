#!/bin/sh
pkill ksiableApi
export GOGIN_MODE=release
export GOPORT=8002
nohup ./bin/ksiableApi & 2>&1