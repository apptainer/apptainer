# Arch for Apptainer

This bootstrap spec will generate an Archlinux distribution using Apptainer.
Note that you can also just bootstrap a Docker image. By default, the metapackage
`base` gets installed. It can be changed using the `Include` parameter (add it
below `Bootstrap`). Similarly, `pacman` configuration can be changed by
specifying a URL to get `pacman.conf` in the `confURL` parameter.

If you want to move forward with the raw, old school, jeans and hard toes
bootstrap, here is what to do. I work on an Ubuntu machine, so I had to use a
Docker Archlinux image to do this. This first part you should do on your local
machine (if not Archlinux) is to use Docker to interactively work in an Archlinux
image. If you don't want to copy paste the build spec file, you can use `--volume`
to mount a directory from your host to a folder in the image (I would recommend
`/tmp` or similar). Here we run the docker image:

```bash
docker run -it  --privileged pritunl/archlinux bash
```

```bash
pacman -S -y git autoconf libtool automake gcc python make sudo vim \
 arch-install-scripts wget apptainer
```

You can add the [Apptainer](Apptainer) build spec here, or cd to where it is
if you have mounted a volume.

```bash
cd /tmp
apptainer create arch.img
sudo apptainer bootstrap arch.img Apptainer
```

That should do the trick!
