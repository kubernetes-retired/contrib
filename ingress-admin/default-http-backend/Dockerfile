FROM index.caicloud.io/caicloud/debian:jessie

# Set the default timezone to Shanghai
RUN echo "Asia/Shanghai" > /etc/timezone
RUN dpkg-reconfigure -f noninteractive tzdata

COPY default-http-backend /
CMD ["./default-http-backend"]