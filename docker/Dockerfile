FROM alpine:3.6

# Update the base image
RUN apk update
RUN apk upgrade

# Add serf binary
COPY ./serf/serf_0.8.1_linux_amd64.zip /tmp/
RUN cd /usr/local/bin && unzip /tmp/serf_0.8.1_linux_amd64.zip && chmod +x serf
