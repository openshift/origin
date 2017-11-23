## containers-storage-shutdown 1 "October 2016"

## NAME
containers-storage shutdown - Shut down layer storage

## SYNOPSIS
**containers-storage** **shutdown** [*options* [...]]

## DESCRIPTION
Shuts down the layer storage driver, which may be using kernel resources.

## OPTIONS
**-f | --force**

Attempt to unmount any mounted layers before attempting to shut down the
driver.  If this option is not specified, if any layers are mounted, shutdown
will not be attempted.

## EXAMPLE
**containers-storage shutdown**
