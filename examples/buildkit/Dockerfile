FROM ubuntu:20.04
RUN apt-get update && apt-get install -y cowsay lolcat && rm -rf /var/lib/apt/lists/*
ENV PATH=/usr/games:$PATH
ENV LC_ALL=C
CMD date | cowsay | lolcat
