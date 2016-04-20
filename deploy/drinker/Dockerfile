FROM debian

ADD drinker /root/drinker
ADD client-config.json /root/.config/camlistore/client-config.json

CMD [ "/root/drinker", "-queue", "root:toor@tcp(queue:3306)/gitarchive", "-influx", "http://influx:8086", "-cache", "/cache/cache.json", "2016-03-26-14" ]

