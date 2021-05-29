default_target: all
.PHONY : default_target

all:
	go build -o bin/w0rpboard cmd/w0rpboard/main.go

clean:
	rm -rf bin

.PHONY: clean
