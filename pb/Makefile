all:clean core.pb.go p2p.pb.go

core.pb.go: core.proto
	protoc \
	--proto_path=../..:. \
	--go_out=./ \
	--go_opt=paths=source_relative \
	--go_opt=Mcore.proto=github.com/wooyang2018/svp-blockchain/pb \
	core.proto

p2p.pb.go: p2p.proto
	protoc \
	--proto_path=../..:. \
	--go_out=./ \
	--go_opt=paths=source_relative \
	--go_opt=Mp2p.proto=github.com/wooyang2018/svp-blockchain/pb \
	p2p.proto

clean:
	rm -f *.pb.go
