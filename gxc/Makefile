.PHONY: test build install clean release

test: build

build:
	go build

install:
	go install

clean:
	rm -f gxc

release:
	$(MAKE) -C .. $@
