make:
	go build smudge.go
	cp smudge /usr/bin/
	yes|cp smudge.service  /etc/systemd/system/ && systemctl daemon-reload && systemctl restart smudge
