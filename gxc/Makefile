.PHONY: test build install clean release

test: build

build:
	go build
	-gxc build-windows-386

install:
	go install

clean:
	rm -f gxc gxc-*

release:
	$(MAKE) -C .. $@
