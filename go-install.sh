#!/bin/bash
BUILD_VERSION=$(git describe --tags --always)
CID=$(git log --format="%H" -n 1)
go install -ldflags "-X main.AppVersion=$BUILD_VERSION -X main.Sha=$CID"