module nailong-lsm-comparison

go 1.21

require (
	github.com/syndtr/goleveldb v1.0.0
	nailong-lsm-go v0.0.0
)

require (
	github.com/BurntSushi/toml v1.3.2 // indirect
	github.com/golang/snappy v0.0.0-20180518054509-2e65f85255db // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	golang.org/x/sys v0.0.0-20220715151400-c0bba94af5f8 // indirect
)

replace nailong-lsm-go => ../..
