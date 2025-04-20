FROM ubuntu:20.04
RUN apt-get update && apt-get install -y cowsay lolcat
ENV PATH=/usr/games:$PATH
ENV LC_ALL=C
ENTRYPOINT date | cowsay | lolcat
