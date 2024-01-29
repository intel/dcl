# Dynamic Compression Library for Intel&reg; technologies - **Alpha version (No Further Development)**

## Status
This project is currently in alpha and will not receive further updates or development.


## Overview
DCL allows an application to automatically use the optimal compression/decompression implementation from those available.  Because no single strategy is always optimal, DCL analyzes the workload and uses its knowledge of the underlying solutions to choose the best path.  Applications use the standard compression APIs and DCL is used transparently. Refer to each of the github's below for information on setup. 

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

## Integration Guide 

### Creating and configuring the Reader/Writer

To compress with the default policy and compression level, you will need to make a new dcl Writer object. The writer needs an io.Writer interface as a parameter where the compressed data will be written. The reader accepts an io.Reader interface that will read out the compressed data. 

```
w, err := dcl.NewWriter(writer)
r, err := dcl.NewReader(reader)
```

By default, the writer will use gzip at a level 1 compression. To change these values, we will use the Apply() method on the writer. 

```
w.Apply(dcl.CompressionLevelOption(5), dcl.AlgorithmOption(dcl.ZSTD))
```
This will change the writer to use compression level 5 and zstd. These same options also work for the Reader object. Any options that are not compatible with a certain compression strategy will disable that strategy, even if they are included in the given policy. 

### Using the Reader/Writer

To use the Reader and writer, we will use the Read and Write methods on the objects. For the writer, this will write the uncompressed data in the buffer into the io.Writer that was used when creating the object. For the reader, this will place the decompressed data into the buffer. 

```
w.Write(buf)
r.Read(buf)
```
To recycle these objects, you can use the Reset() method to change the io.Reader and io.Writer that the objects are using. Also, there is a method for closing the reader/writer when they are no longer needed. 

```
w.Reset(newWriter)
w.Close()
```

### Creating a policy for the Reader/Writer

You can manually set how the different strategies are selected by creating a policy for the Reader/Writer. It is recommended to do this for your writer based on the performance you see of the different strategies. A future update would include an automated tool to benchmark the best values for your specific hardware. Below is an example policy that can be created..

```
func BufferSizePolicy(params *dcl.PolicyParameters) []dcl.StrategyType {
	var list []dcl.StrategyType
	if params.BufferSize < 1500 {
		list = []StrategyType{ISAL, IAA, QAT}
	} else {
		list = []StrategyType{QAT, IAA, ISAL}
	}
	return list
}
...
w, err := dcl.NewWriter(writer)
w.SetPolicy(BufferSizePolicy)
```
* The current available parameters are:
  * algorithm
  * buffer size
  * compression level
  * Compress/decompress

## Contributions and Forks

While active development has ceased, we welcome the community to fork this project and build upon it. If you have any questions or wish to discuss potential uses or modifications, feel free to place these in the github issues section of the project.

## Known Issues

* Using zstd and gzip through QAT on the same process can cause segmentation faults within the qat driver.
* Consecutive writes using a single ISA-L writer can cause segmentation faults, and this project contains a workaround to use a new writer each compress. 
