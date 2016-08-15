FROM busybox

ADD . /contrib

CMD ["/bin/true"]

VOLUME ["/contrib"]
