# hmrc-lib-upgrade

TravisCI status: [![Build Status](https://travis-ci.org/thoeni/hmrc-lib-upgrade.svg?branch=v2.1.2)](https://travis-ci.org/thoeni/hmrc-lib-upgrade)

## Usage

### Flags
- `-h` shows help
- `-version` shows version and commit of the executable
- `-file` allows to specify which `.*Build.scala` input file to use (you might have either `MicroServiceBuild.scala` or `FrontendBuild.scala`)
- `-migration` will show libraries to be removed as part of [PlatOps migration](https://confluence.tools.tax.service.gov.uk/x/wJFhBQ)

### Build
If you have the Go compiler, you can build with:
```
./build.sh [linux|windows|mac]
``` 
depending on the platform. Omitting the parameter will build three versions.

Under [GitHub Releases](https://github.com/thoeni/hmrc-lib-upgrade/releases) you can find the built version for the latest release.

#### Examples
```
hmrc-lib-upgrade -file=project/MicroServiceBuild.scala -migration
```
