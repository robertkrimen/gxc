.PHONY: test release install build clean

test:
	$(MAKE) -C gxc $@

release: test
	(cd gxc && godocdown -signature . > README.markdown) || false
	cp gxc/README.markdown .

install: test
	$(MAKE) -C gxc $@
	go install

build:
	$(MAKE) -C gxc $@

clean:
	$(MAKE) -C gxc $@
