#!/bin/sh

filter=${ENV_FILTER:-}

sedfiltername=./sample.d/sample.sed

containerNames(){
	docker \
		ps \
		--format '{{.Names}}' \
		--filter=name="${filter}" |
		sed \
			-n \
			-f "${sedfiltername}" |
		sort
}

containerNames
containerNames | xargs ./docker-logs4containers
