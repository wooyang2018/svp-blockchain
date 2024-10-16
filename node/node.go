// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package node

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path"
	"runtime"
	"syscall"

	"github.com/multiformats/go-multiaddr"
	"go.uber.org/zap"

	"github.com/wooyang2018/svp-blockchain/consensus"
	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/execution"
	"github.com/wooyang2018/svp-blockchain/logger"
	"github.com/wooyang2018/svp-blockchain/p2p"
	"github.com/wooyang2018/svp-blockchain/storage"
	"github.com/wooyang2018/svp-blockchain/txpool"
)

type Node struct {
	config Config

	privKey *core.PrivateKey
	peers   []*Peer
	genesis *Genesis

	roleStore *roleStore
	storage   *storage.Storage
	host      *p2p.Host
	msgSvc    *p2p.MsgService
	txpool    *txpool.TxPool
	execution *execution.Execution
	consensus *consensus.Consensus
}

func Run(config Config) {
	node := new(Node)
	node.config = config
	node.setupDirs()
	node.setupLogger()
	node.readFiles()
	node.limitCPUs()
	node.setupComponents()
	logger.I().Infow("node setup done")
	node.consensus.Start()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	logger.I().Info("node killed")
	node.consensus.Stop()
	node.host.Close()
}

func (node *Node) limitCPUs() {
	if consensus.PreserveTxFlag {
		runtime.GOMAXPROCS(MaxProcsNum)
		logger.I().Debugf("setup node to use %d out of %d CPUs",
			runtime.GOMAXPROCS(0), runtime.NumCPU())
	}
}

func (node *Node) setupLogger() {
	var inst *zap.Logger
	var err error
	if node.config.Debug {
		inst, err = zap.NewDevelopment()
	} else {
		inst, err = zap.NewProduction()
	}
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}
	logger.Set(inst.Sugar())
}

func (node *Node) setupDirs() {
	node.config.ExecutionConfig.BinccDir = path.Join(node.config.DataDir, "bincc")
	os.Mkdir(node.config.ExecutionConfig.BinccDir, 0755)
	node.config.ExecutionConfig.ContractDir = path.Join(node.config.DataDir, "contracts")
	os.Mkdir(node.config.ExecutionConfig.ContractDir, 0755)
}

func (node *Node) readFiles() {
	var err error
	node.privKey, err = ReadNodeKey(node.config.DataDir)
	if err != nil {
		logger.I().Fatalw("read key failed", "error", err)
	}
	logger.I().Infow("read nodekey", "pubkey", node.privKey.PublicKey())

	node.genesis, err = ReadGenesis(node.config.DataDir)
	if err != nil {
		logger.I().Fatalw("read genesis failed", "error", err)
	}

	node.peers, err = ReadPeers(node.config.DataDir)
	if err != nil {
		logger.I().Fatalw("read peers failed", "error", err)
	}
	logger.I().Infow("read peers", "count", len(node.peers))
}

func (node *Node) setupComponents() {
	node.setupRoleStore()
	node.setupHost()
	node.host.SetLeader(0)
	node.msgSvc = p2p.NewMsgService(node.host)
	node.storage = storage.New(path.Join(node.config.DataDir, "db"), node.config.StorageConfig)
	node.execution = execution.New(node.storage, node.config.ExecutionConfig)
	node.txpool = txpool.New(node.storage, node.execution, node.msgSvc, node.config.BroadcastTx)
	node.setupConsensus()
	node.setReqHandlers()
	serveNodeAPI(node)
}

func (node *Node) setupRoleStore() {
	node.roleStore = NewRoleStore(node.genesis, node.peers)
	core.SetSRole(node.roleStore)
	logger.I().Infow("setup role store", "window size", node.roleStore.GetWindowSize())
}

func (node *Node) setupHost() {
	checkPort(node.config.PointPort)
	checkPort(node.config.TopicPort)
	pointAddr, _ := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", node.config.PointPort))
	topicAddr, _ := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", node.config.TopicPort))

	host, err := p2p.NewHost(node.privKey, pointAddr, topicAddr)
	if err != nil {
		logger.I().Fatalw("cannot create p2p host", "error", err)
	}
	node.roleStore.SetHost(host)
	host.SetRoleStore(node.roleStore)

	if err = host.JoinChatRoom(); err != nil {
		logger.I().Errorw("failed to join chatroom", "error", err)
	}
	node.host = host
	logger.I().Infow("setup p2p host", "point port",
		node.config.PointPort, "topic port", node.config.TopicPort)
}

func checkPort(port int) {
	ln, err := net.Listen("tcp4", fmt.Sprintf(":%d", port))
	if err != nil {
		logger.I().Fatalf("cannot listen on port %d\n", port)
	}
	ln.Close()
}

func (node *Node) setupConsensus() {
	node.config.ConsensusConfig.DataDir = node.config.DataDir
	node.consensus = consensus.New(&consensus.Resources{
		Signer:    node.privKey,
		RoleStore: node.roleStore,
		Storage:   node.storage,
		MsgSvc:    node.msgSvc,
		Host:      node.host,
		TxPool:    node.txpool,
		Execution: node.execution,
	}, node.config.ConsensusConfig)
}

func (node *Node) setReqHandlers() {
	node.msgSvc.SetReqHandler(&p2p.BlockReqHandler{
		GetBlock: node.GetBlock,
	})
	node.msgSvc.SetReqHandler(&p2p.QCReqHandler{
		GetQC: node.GetQC,
	})
	node.msgSvc.SetReqHandler(&p2p.BlockByHeightReqHandler{
		GetBlockByHeight: node.storage.GetBlockByHeight,
	})
	node.msgSvc.SetReqHandler(&p2p.TxListReqHandler{
		GetTxList: node.GetTxList,
	})
}

func (node *Node) GetBlock(hash []byte) (*core.Block, error) {
	if blk := node.consensus.GetBlock(hash); blk != nil {
		return blk, nil
	}
	return node.storage.GetBlock(hash)
}

func (node *Node) GetQC(hash []byte) (*core.QuorumCert, error) {
	if blk := node.consensus.GetQC(hash); blk != nil {
		return blk, nil
	}
	return node.storage.GetQC(hash)
}

func (node *Node) GetTxList(hashes [][]byte) (*core.TxList, error) {
	ret := make(core.TxList, len(hashes))
	for i, hash := range hashes {
		tx := node.txpool.GetTx(hash)
		if tx != nil {
			ret[i] = tx
			continue
		}
		tx, err := node.storage.GetTx(hash)
		if err != nil {
			return nil, err
		}
		ret[i] = tx
	}
	return &ret, nil
}
