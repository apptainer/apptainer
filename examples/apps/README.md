# Apptainer SCI-F Apps

The Scientific Filesystem is well suited for Apptainer containers to
allow you to build a container that has multiple entrypoints, along with
modular environments, libraries, and executables. Here we will review
the basic building and using of a Apptainer container that implements
SCIF. For more quick start tutorials, see the [official documentation
for SCIF](https://vsoch.github.io/scif/).

Build your image

```sh
sudo apptainer build cowsay.simg Apptainer.cowsay 
```

What apps are installed?

```console
$ apptainer apps cowsay.simg
cowsay
fortune
lolcat
```

Ask for help for a specific app!

```console
$ apptainer help --app fortune cowsay.simg
fortune is the best app
```

Run a particular app

```console
$ apptainer run --app fortune cowsay.simg
When I reflect upon the number of disagreeable people who I know who have gone
to a better world, I am moved to lead a different life.
    -- Mark Twain, "Pudd'nhead Wilson's Calendar"
```

Inspect an app

```console
$ apptainer inspect --app fortune cowsay.img 
{
    "SCIF_APPNAME": "fortune",
    "SCIF_APPSIZE": "1MB"
}
```
