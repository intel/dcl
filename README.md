# Dynamic Compression Library for Intel&reg; technologies - **Alpha version (No Further Development)**

## Status
This project is currently in alpha and will not receive further updates or development.


## Overview
dcl provides a dynamic environment that selects the optimal compression/decompression technologies that are best for the type of workload. Refer to each of the github's below for information on setup. 

## Underlying Technologies

* QAT: https://github.com/intel/qatgo 
* ISA-L: https://github.com/intel/ISALgo
* IAA: https://github.com/intel/ixl-go

## Supported Algorithms

* Zstd: QAT (ISA-L is coming in a future update)
* Gzip: QAT, IAA, ISA-L
* LZ4: QAT

Each of these algorithms have a software backup that is used if the other solutions are not available

## Software Requirements

* Go 1.20 or above
* QATzip library 1.1.2 or above: https://github.com/intel/QATzip
  * Requirements for QATzip: https://github.com/intel/QATzip/blob/master/README.md#software-requirements
* Optional: Intel zstd QAT Plugin (required for zstd): https://github.com/intel/QAT-ZSTD-Plugin
  * Optional: libzstd v1.5.5 (required for zstd plugin): https://github.com/facebook/zstd
* ISA-L library: https://github.com/intel/isa-l

These are currently all required to be installed during beta, but will only require the solutions you need on version 1 release.

## Contributions and Forks

While active development has ceased, we welcome the community to fork this project and build upon it. If you have any questions or wish to discuss potential uses or modifications, feel free to place these in the github issues section of the project.

## Known Issues

* Using zstd and gzip through QAT on the same process can cause segmentation faults within the qat driver.
* Consecutive writes using a single ISA-L writer can cause segmentation faults, and this project contains a workaround to use a new writer each compress. 

## Notes

You can manually set how the technologies are chosen based off of a set of paramaters. For example:

```
func BufferSizePolicy(params *PolicyParameters) []strategyType {
	var list []strategyType
	if params.bufferSize < 1500 {
		list = []strategyType{ISAL, IAA, QAT}
	} else {
		list = []strategyType{QAT, IAA, ISAL}
	}
	return list
}
```
* The current available parameters are:
  * algorithm
  * buffer size
  * compression level
  * Compress/decompress

