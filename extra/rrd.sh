#!/usr/bin/env bash

set -ueo pipefail

for room in bedroom office; do 
    INFILE="airthingy-${room?}.json"
    RRD="${room?}.rrd"

    MIN=10
    HOUR=$((4 * 60))
    SIXM=$((60 * 24 * 180))

    if true; then
	echo "Creating rrd file…"
	rm -f "${RRD?}"
	rrdtool create "${RRD?}" \
		--start "$(date +%s -d2023-01-01)" \
		-s 900 \
		DS:humidity:GAUGE:3600:1:100000 \
		DS:radonshort:GAUGE:3600:1:10000000 \
		DS:radonlong:GAUGE:3600:1:100000 \
		DS:temperature:GAUGE:3600:1:10000000 \
		DS:pressure:GAUGE:3600:1:10000 \
		DS:co2:GAUGE:3600:1:10000000 \
		DS:voc:GAUGE:3600:1:10000 \
		RRA:AVERAGE:0:${MIN?}:${SIXM?} \
		RRA:AVERAGE:0:${HOUR?}:${SIXM?}
    fi

    if true; then
	jq -cr '.timestamp, .humidity*100, .radon_short, .radon_long, .temperature*100, .pressure*100, .co2, .voc' "${INFILE?}" \
	| xargs -n 8 \
	| sed 's/ /:/g'  \
	| xargs rrdtool update "${RRD?}"
    fi
done

echo "Graphing…"
for col in co2 voc; do
    rrdtool graph "${col?}.png" \
            -X 0 \
            -t "${col?}" \
            -l -1 \
            -w 1280 \
            -h 720 \
            -s -3d \
            -e now \
            -a PNG \
            "DEF:office=office.rrd:${col?}:AVERAGE:step=3600:reduce=AVERAGE" \
            "DEF:bedroom=bedroom.rrd:${col?}:AVERAGE:step=3600:reduce=AVERAGE" \
            "LINE2:office#ff0000:Office" \
            "LINE2:bedroom#0000ff:Bedroom"
done
for col in temperature humidity; do
    rrdtool graph "${col?}.png" \
            -X 0 \
            -t "${col?}" \
            -l 0 \
            -w 1280 \
            -h 720 \
            -s -3d \
            -e now \
            -a PNG \
            "DEF:office=office.rrd:${col?}:AVERAGE:step=3600:reduce=AVERAGE" \
            "DEF:bedroom=bedroom.rrd:${col?}:AVERAGE:step=3600:reduce=AVERAGE" \
	    'CDEF:b=bedroom,100,/' \
	    'CDEF:o=office,100,/' \
            'LINE2:o#ff0000:Office' \
            'LINE2:b#0000ff:Bedroom'
done
