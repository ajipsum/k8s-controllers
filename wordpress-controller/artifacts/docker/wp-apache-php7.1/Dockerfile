FROM php:7.1-apache

# install the PHP extensions we need
RUN set -ex; \
	\
	savedAptMark="$(apt-mark showmanual)"; \
	\
	apt-get update; \
	apt-get install -y --no-install-recommends \
		libjpeg-dev \
		libpng-dev \
	; \
	\
	docker-php-ext-configure gd --with-png-dir=/usr --with-jpeg-dir=/usr; \
	docker-php-ext-install gd mysqli opcache zip; \
	\
# reset apt-mark's "manual" list so that "purge --auto-remove" will remove all build dependencies
	apt-mark auto '.*' > /dev/null; \
	apt-mark manual $savedAptMark; \
	ldd "$(php -r 'echo ini_get("extension_dir");')"/*.so \
		| awk '/=>/ { print $3 }' \
		| sort -u \
		| xargs -r dpkg-query -S \
		| cut -d: -f1 \
		| sort -u \
		| xargs -rt apt-mark manual; \
	\
	apt-get purge -y --auto-remove -o APT::AutoRemove::RecommendsImportant=false; \
	rm -rf /var/lib/apt/lists/*

# set recommended PHP.ini settings
# see https://secure.php.net/manual/en/opcache.installation.php
RUN { \
		echo 'opcache.memory_consumption=128'; \
		echo 'opcache.interned_strings_buffer=8'; \
		echo 'opcache.max_accelerated_files=4000'; \
		echo 'opcache.revalidate_freq=2'; \
		echo 'opcache.fast_shutdown=1'; \
		echo 'opcache.enable_cli=1'; \
	} > /usr/local/etc/php/conf.d/opcache-recommended.ini
# https://codex.wordpress.org/Editing_wp-config.php#Configure_Error_Logging
RUN { \
		echo 'error_reporting = 4339'; \
		echo 'display_errors = Off'; \
		echo 'display_startup_errors = Off'; \
		echo 'log_errors = On'; \
		echo 'error_log = /dev/stderr'; \
		echo 'log_errors_max_len = 1024'; \
		echo 'ignore_repeated_errors = On'; \
		echo 'ignore_repeated_source = Off'; \
		echo 'html_errors = Off'; \
	} > /usr/local/etc/php/conf.d/error-logging.ini

RUN a2enmod rewrite expires

VOLUME /var/www/html

ENV WORDPRESS_VERSION 5.1.1
ENV WORDPRESS_SHA1 f1bff89cc360bf5ef7086594e8a9b68b4cbf2192

ENV WORDPRESS_DB_HOST websites.cujxcm1nmul2.us-east-1.rds.amazonaws.com
ENV WORDPRESS_DB_USER root
ENV WORDPRESS_DB_PASSWORD client01
ENV WORDPRESS_DB_NAME wordpress
ENV WORDPRESS_TABLE_PREFIX WP_

ENV AUTH_KEY RU5UOEBLUENENlNKJzkpaE1TKSx4V0tQSShvYENDbW5ZVkZoQEt9TE5TLzIwOjhPZGY5JSc2YTReY3p4a1g0LQ==
ENV SECURE_AUTH_KEY MXJRLlZZQUtdcyk7KX5bKnskeixBe20uN3ZeXzFCby5TbU1Jdkh1VSFGSn10bjNwUWYmUmBdcylbMk9Tajp2LQ==
ENV LOGGED_IN_KEY Sm4xdmVGSV0zP0pmPVUsN3llX1lacChvP2M3YWtlYiVEX3JZbk4jKEo+QGVAL1ZUdHd5aVlKbEBWaT1zcj11PQ==
ENV NONCE_KEY Jm9eTn1oNTRKLn5pOTlbWHpqQ3tNJG53YVgneEFGPW1PPiYwdSRDVTFPLlB1RVc1NVZtcmRPfWMoYG43QmNNdg==
ENV AUTH_SALT RnorP35BZFB3d3IwfVNsWnYsL0szZiE3Xj0tV2NQY20wc1IxZFpob2dfNnFqXn1tLjpDUmQzXV5XZCh3W1ApPw==
ENV SECURE_AUTH_SALT QllpZVlrfkExRzFnWlpMUDI/aXJieyVifW0yP2BUP3Z0ZHwwJCFNS0klbEUsNGBUbWhjeE4sNVNge2Mzbzt5IQ==
ENV LOGGED_IN_SALT QHNeZSJaSTU0PnNWKS9QOjxmWConIVRhMmwuWFpHO2QxV0xxQzBdY0NeTjFpNiU+PCEnYEA1ZT5KSy5oanl4Ug==
ENV NONCE_SALT OCclcHg7UTIzZXdkVi9PJCsoVWZTWkUuTUwnZ2N3IzFPfGpZRWBAaD5VMTtbdnIzWHtSdnt4ZUVWcGtETnhzUQ==

RUN set -ex; \
	curl -o wordpress.tar.gz -fSL "https://wordpress.org/wordpress-${WORDPRESS_VERSION}.tar.gz"; \
	echo "$WORDPRESS_SHA1 *wordpress.tar.gz" | sha1sum -c -; \
# upstream tarballs include ./wordpress/ so this gives us /usr/src/wordpress
	tar -xzf wordpress.tar.gz -C /usr/src/; \
	rm wordpress.tar.gz; \
	chown -R www-data:www-data /usr/src/wordpress

COPY docker-entrypoint.sh /usr/local/bin/

ENTRYPOINT ["docker-entrypoint.sh"]
CMD ["apache2-foreground"]
