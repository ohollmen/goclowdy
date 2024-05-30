# GoClowdy

Access and automate chores related to VMs and Machine Images.

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

## Building Main Executable

This should build the main executable by name 'goclowdy':
```
go build
```
Code it short and it will (in future) contain features that lead to deletion of machine images -
please review and understand the code before running it.

## Configuration file

Config file supports following config variables (for MIs):

- Project - The GCP Project to scope operation to (Can be overriden by --project CL parameter for most operations)
- CredF - Credentials file (GCP service account JSON key, be sure to have only user access to this file)
- DelOK - Allow deleting machine images (default: false). This also works as a flag for allowing to create a new MI (machine image)
  by same name (by deleting old first)
- Debug - Increase verbosity for runtime messages (default: false)
- NameREStr - Machine image "standard name" REgExp String (default: none, must be valid RE). If pattern is set, **only the machine images
  that match this pattern will be considered for deletion (MI:s that do not are always kept).
  When this string is empty / not present, filtering 
- HostREStr - Machine image name pattern with 2 RE capture groups (i.e. "(...)...(...)" ) for deriving:
  - what VM MI belongs to
  - when the MI was created (ISO time)
- Machine image Retention time config vars (all times given as **hours**):
  - KeepMinH - Keep minumum time (all MIs below this age get kept, default: 168 hrs => 7 days/1 week)
  - KeepMaxH - Keep maximum time (all MIs above this age get deleted, default: 133320 hrs => 78(+) weeks)
  - WD_keep - The weekday on which to keep (or take) the backup (for weekly backup)
  - MD_keep - The day-of-the-month on which to keep (or take) the backup (for montly backup)
- StorLoc - Storage location for creating MI:s in (default: none, e.g. "us" for later us-wide multiregion access)
- ChunkDelSize - Chunk size for chunked deletion (or trigger work-queue based deletion by using value -1, See also WorkerLimit)
- WorkerLimit - The number of concurrent workers that should be maintained when using work-queue

### Overriding settings from environment (and CL)

The following config vars can be override by environment or by CL params:

| Config var | Env. Var.   | Notes |
|------------|-------------|-------|
| CredF      | GOOGLE_APPLICATION_CREDENTIALS | |
| Project    | GCP_PROJECT | |
| NameREStr  | MI_STDNAME  | No cap groups |
| HostREStr  | MI_HOSTPATT | 2 cap groups for Hname, CTime |
| NA | GCP_VM_NAMEPATT | For |
| DelOK      | MI_DELETE_EXEC | |
| ChunkDelSize | MI_CHUNK_DEL_SIZE | |

## Running the code

Executable grsc currently supports 2 subcommands:

- vmlist - List virtual machines in project and revirew their current backups
- milist - list machine images and classify them as keepers or to-be-deleted based on simple backup retainment policy.

# TODO

- Enahance documentation, the config options of modules/ CLI utils.
- Add subcomands / use cases to CLI utility

