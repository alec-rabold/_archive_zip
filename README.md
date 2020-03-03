# Zipspy
zipspy is a CLI tool to extract files from zip archives in S3 without needing to download the entire archive

<!-- TOC depthFrom:1 depthTo:2 withLinks:1 updateOnSave:1 orderedList:0 -->

- [Zipspy CLI](#zipspy)
    - [Preamble](#preamble)
    - [Sample Flow](#sample-flow)
    

<!-- /TOC -->



## Preamble

The zipspy CLI allows you to download specific files and/or directories from zip archives in S3 without having to download the entire object. 


## Sample Flow

Be sure to set your AWS environment variables before using zipspy:

`export AWS_PROFILE={profile}`  
`export AWS_DEFAULT_REGION={region}`

The following is an example of how to use the CLI, downloading a file called `plan.txt` from an S3 zip archive called `archive.zip` in a bucket named `zipspy-extractor-test`. By default, the result is printed to stdout.

```
zipspy extract -b zipspy-extractor-test -k archive.zip -f plan.txt
```

You can specify multiple files and/or files paths. Zipspy will download all files whose filepaths contain the given input string. For example:

With an `archive.zip` that has the following structure:

```
.
├── plan.txt
├── foldername1
|   ├── plan.txt
|   └── technology.md
├── foldername2
|   ├── plan.txt
|   └── header.html
```


`zipspy extract -b zipspy-extractor-test -k archive.zip -f foldername2 -f plan.txt` will download the following files:

`archive/foldername1/plan.txt`  
`archive/foldername2/plan.txt`  
`archive/foldername2/header.html`  


You may also specify output paths to write the file content to. By default, downloaded data will be appended to the specified file(s). If they don't exist, zipspy will create them.

The following example demonstrates how to download a file named `plan.txt` and save it to `data/my-plan.txt`.

```
zipspy extract -b zipspy-extractor-test -k archive.zip -f plan.txt -o data/my-plan.txt
```

You may specify any number of output files as long as there is an equal number of files to download.

```
zipspy extract -b zipspy-extractor-test -k archive.zip -f plan.txt -o data/my-plan.txt -f file.md -o data/file.md
```
