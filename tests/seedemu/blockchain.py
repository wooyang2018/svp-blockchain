import os
from typing import Dict, Tuple, List

import yaml
from seedemu.core import Emulator, Layer, Node

ReplaceFileTemplate = """\
#!/bin/bash
i=0
while read -r replacement || [[ -n "$replacement" ]]; do 
    sed -E -i "s/dns4\/{}$i/ip4\/$replacement/g" {}
    ((i++))
done < {}
"""


class PPoV(Layer):
    nodePrefix: str
    nodeCount: int
    dataDir: str
    vpnodes: Dict[Tuple[int, str], int]
    addrs: List[str]
    commands: List[str]

    def __init__(self, nodePrefix: str, nodeCount: int, dataDir: str):
        super().__init__()
        self.nodePrefix = nodePrefix
        self.nodeCount = nodeCount
        self.dataDir = dataDir
        self.vpnodes = {}
        self.addrs = [''] * nodeCount
        self.commands = [''] * nodeCount
        self.addDependency('Base', False, False)

    def getName(self) -> str:
        return 'PPoV'

    def configure(self, emulator: Emulator):
        assert len(self.vpnodes) == self.nodeCount, 'PPoV::configure'
        for ((scope, type, name), obj) in emulator.getRegistry().getAll().items():
            if type != 'hnode' and type != 'rnode': continue
            node: Node = obj
            if (node.getAsn(), node.getName()) in self.vpnodes:
                vnode = self.vpnodes[(node.getAsn(), node.getName())]
                self.addrs[vnode] = str(node.getInterfaces()[0].getAddress())

        composePath = '../workdir/local-clusters/docker_template/docker-compose.yaml'
        outputPath = os.path.join(self.dataDir, 'output.log')
        with open(composePath, 'r') as f:
            services = yaml.safe_load(f).get('services', {})
            for name, config in services.items():
                vnode = int(name[len(self.nodePrefix):])
                self.commands[vnode] = config.get('command') + ' > ' + outputPath + ' 2>&1 &'

    def render(self, emulator: Emulator) -> None:
        chainPath = os.path.join(self.dataDir, 'chain')
        peersPath = os.path.join(self.dataDir, 'peers.json')
        addrsPath = os.path.join(self.dataDir, 'addrs')
        replacePath = os.path.join(self.dataDir, 'replace.sh')

        for ((scope, type, name), obj) in emulator.getRegistry().getAll().items():
            if type != 'hnode' and type != 'rnode': continue
            node: Node = obj
            if (node.getAsn(), node.getName()) in self.vpnodes:
                vnode = self.vpnodes[(node.getAsn(), node.getName())]
                templateDir = os.path.join('../../../workdir/local-clusters/docker_template/', str(vnode))
                self._log('rendering as{}/{} as {}{}...'.format(node.getAsn(), node.getName(), self.nodePrefix, vnode))

                node.importFile('../../../chain', chainPath)
                node.importFile(os.path.join(templateDir, 'genesis.json'), os.path.join(self.dataDir, 'genesis.json'))
                node.importFile(os.path.join(templateDir, 'nodekey'), os.path.join(self.dataDir, 'nodekey'))
                node.importFile(os.path.join(templateDir, 'peers.json'), peersPath)
                node.setFile(addrsPath, '\n'.join(self.addrs))
                node.setFile(replacePath, ReplaceFileTemplate.format(self.nodePrefix, peersPath, addrsPath))

                node.appendStartCommand('chmod +x ' + replacePath)
                node.appendStartCommand(replacePath)
                node.appendStartCommand('chmod +x ' + chainPath)
                node.appendStartCommand(self.commands[vnode])

    def bindNode(self, asn: int, pnode: str, vnode: int):
        assert 0 <= vnode < self.nodeCount, 'PPoV::bindNode'
        self.vpnodes[(asn, pnode)] = vnode
