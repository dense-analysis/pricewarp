default_target: all
.PHONY : default_target

all:
	go build -o bin/pricewarp cmd/pricewarp/main.go

clean:
	rm -rf bin

.PHONY: clean
