#!/usr/bin/env python3
# encoding: utf-8

from seedemu import *

from blockchain import PPoV

# Dimension of hypercube topology
dimension = 3

# Initialize the emulator and layers
emu = Emulator()
base = Base()
routing = Routing()
ospf = Ospf()
ppov = PPoV('node', 2 ** dimension, '/app')

# Create an autonomous system
as150 = base.createAutonomousSystem(150)

# Create d*2^(d-1) networks
# eg. net00-01,...,node10-11
for i in range(0, 2 ** dimension - 1):
    for j in range(i + 1, 2 ** dimension):
        diff = i ^ j
        if diff == 1 << (diff.bit_length() - 1):
            as150.createNetwork(
                'net' + bin(i).replace('0b', '').zfill(dimension) + '-' + bin(j).replace('0b', '').zfill(dimension))

docker = Docker(internetMapEnabled=True)
image = DockerImage(name='chain_bin', local=True, software=[])
docker.addImage(image)

# Create 2^d rnodes
# eg. node00,...,node11
netList = as150.getNetworks()
for i in range(0, 2 ** dimension):
    nodeIndex = bin(i).replace('0b', '').zfill(dimension)
    node = as150.createRouter('node' + nodeIndex)
    ppov.bindNode(150, 'node' + nodeIndex, i)  # Bind ppov node to rnode
    docker.setImageOverride(node, 'chain_bin')
    for netItem in netList:
        if nodeIndex in netItem:
            node.joinNetwork(netItem)

# Render
emu.addLayer(base)
emu.addLayer(routing)
emu.addLayer(ospf)
emu.addLayer(ppov)
emu.render()

# Generate the Docker files
outputDir = 'output'
emu.compile(docker, outputDir, True)

# Copy the image folder to the output folder
os.system('mkdir ' + outputDir + '/chain_bin/')
os.system('cp -r Dockerfile ' + outputDir + '/chain_bin/')
os.system('cp -r ../chain ' + outputDir + '/chain_bin/')
