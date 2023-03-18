# GoClowdy

Access and automate chores related to VM and Machine Images.

goclowdy provides easy-to-use Go wrappers for accessing the inventory of VM:s
and VM Machine images with a very terse and compact wrapper code for the
parts that otherwise cause a lot of code indirection and verbosity with GCP
native APIs. It can equally be used to build CLI apps, web apps and cloud functions.

## Pre-requisites

- Go runtime and compiler (Version 1.16 preferred)
- Make utility

Set environment variables for project and JSON credentials:
- GCP_PROJECT - The GCP Project from where the GCP artifacts are going to be accessed
- GOOGLE_APPLICATION_CREDENTIALS - The JSON credentials for accessing CGP org/project.

## Building Test Executable

```
go build grsc.go
```
Code it short and it will (in future) contain features that lead to deletion of machine images -
please review and understand the code before running it.

## Running the code

Executable grsc currently supports 2 subcommands:

- vmlist - List virtual machines in project and revirew their current backups
- milist - list machine images and classify them as keepers or to-be-deleted based on simple backup retainment policy.

# TODO

- Enahance documentation, the config options of modules/ CLI utils.
- Add subcomands / use cases to CLI utility

