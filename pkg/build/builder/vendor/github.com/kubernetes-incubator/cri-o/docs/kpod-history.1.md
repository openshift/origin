% kpod(1) kpod-history - Simple tool to view the history of an image
% Urvashi Mohnani
% kpod-history "1" "JULY 2017" "kpod"

## NAME
kpod-history - Shows the history of an image

## SYNOPSIS
**kpod history [OPTIONS] IMAGE[:TAG|DIGEST]**

## DESCRIPTION
**kpod history** displays the history of an image by printing out information
about each layer used in the image. The information printed out for each layer
include Created (time and date), Created By, Size, and Comment. The output can
be truncated or not using the **--no-trunc** flag. If the **--human** flag is
set, the time of creation and size are printed out in a human readable format.
The **--quiet** flag displays the ID of the image only when set and the **--format**
flag is used to print the information using the Go template provided by the user.

Valid placeholders for the Go template are listed below:

| **Placeholder** | **Description**                                                               |
| --------------- | ----------------------------------------------------------------------------- |
| .ID             | Image ID                                                                      |
| .Created        | if **--human**, time elapsed since creation, otherwise time stamp of creation |
| .CreatedBy      | Command used to create the layer                                              |
| .Size           | Size of layer on disk                                                         |
| .Comment        | Comment for the layer                                                         |

**kpod [GLOBAL OPTIONS]**

**kpod [GLOBAL OPTIONS] history [OPTIONS]**

## GLOBAL OPTIONS

**--help, -h**
  Print usage statement

## OPTIONS

**--human, -H**
    Display sizes and dates in human readable format

**--no-trunc**
    Do not truncate the output

**--quiet, -q**
    Print the numeric IDs only

**--format**
    Alter the output for a format like 'json' or a Go template.


## COMMANDS

**kpod history debian**

**kpod history --no-trunc=true --human=false debian**

**kpod history --format "{{.ID}} {{.Created}}" debian**

**kpod history --format json debian**

## history
Show the history of an image

## SEE ALSO
kpod(1), crio(8), crio.conf(5)

## HISTORY
July 2017, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
