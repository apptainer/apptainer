# Apptainer example plugin

This directory contains an example CLI plugin for apptainer. It demonstrates
how to add a command and flags.

## Building

In order to build the plugin you need a copy of code matching the version of
apptainer that you wish to use. You can find the commit matching the
apptainer binary by running:

```console
$ apptainer version
1.0.0-23.g-00d15ea5e
```

this means this version of apptainer is _post_ 1.0.0 by 23 commits (but before the
next version after that one). The suffix .gXXXXXXXXX indicates the exact
commit in github.com/apptainer/apptainer used to build this binary
(00d15ea5e in this example).

Obtain a copy of the source code by running:

```sh
git clone https://github.com/apptainer/apptainer.git
cd apptainer
git checkout 00d15ea5e
```

Still from within that directory, run:

```sh
apptainer plugin compile ./examples/plugins/cli-plugin
```

This will produce a file `./examples/plugins/cli-plugin/cli-plugin.sif`.

Currently there's a limitation regarding the location of the plugin code: it
must reside somewhere _inside_ the apptainer source code tree.

## Installing

Once you have compiled the plugin into a SIF file, you can install it into the
correct apptainer directory using the command:

```sh
sudo apptainer plugin install ./examples/plugins/cli-plugin/cli-plugin.sif
```

Apptainer will automatically load the plugin code from now on.

## Other commands

You can query the list of installed plugins:

```console
$ apptainer plugin list
ENABLED  NAME
    yes  example.com/cli-plugin
```

Disable an installed plugin:

```sh
sudo apptainer plugin disable example.com/cli-plugin
```

Enable a disabled plugin:

```sh
sudo apptainer plugin enable example.com/cli-plugin
```

Uninstall an installed plugin:

```sh
sudo apptainer plugin uninstall example.com/cli-plugin
```

And inspect a SIF file before installing:

```console
$ apptainer plugin inspect examples/plugins/cli-plugin/cli-plugin.sif
Name: example.com/cli-plugin
Description: This is a short example CLI plugin for Apptainer
Author: Apptainer Team
Version: 0.1.0
```
