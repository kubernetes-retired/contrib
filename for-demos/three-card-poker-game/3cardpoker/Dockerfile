FROM php:5-apache

RUN apt-get update
RUN apt-get install -y php-pear
RUN pear channel-discover pear.nrk.io

# If the container's stdio is connected to systemd-journald,
# /proc/self/fd/{1,2} are Unix sockets and apache will not be able to open()
# them. Use "cat" to write directly to the already opened fds without opening
# them again.
RUN sed -i 's#ErrorLog /proc/self/fd/2#ErrorLog "|$/bin/cat 1>\&2"#' /etc/apache2/apache2.conf
RUN sed -i 's#CustomLog /proc/self/fd/1 combined#CustomLog "|/bin/cat" combined#' /etc/apache2/apache2.conf

ADD server.php /var/www/html/server.php
ADD controllers.js /var/www/html/controllers.js
ADD index.html /var/www/html/index.html
ADD bootstrap.min.css /var/www/html/bootstrap.min.css
ADD angular.min.js /var/www/html/angular.min.js
ADD ui-bootstrap-tpls.js /var/www/html/ui-bootstrap-tpls.js
ADD ./PNGcards/* /var/www/html/PNGcards/
