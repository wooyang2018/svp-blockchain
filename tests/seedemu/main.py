#!/usr/bin/env python3
# encoding: utf-8

from seedemu.compiler import Docker
from seedemu.core import Emulator
from seedemu.layers import Base

from blockchain import PPoV

###############################################################################
# Initialize the emulator and layers
emu = Emulator()
base = Base()
ppov = PPoV("node", 4, "/app")

###############################################################################
# Create an autonomous system
as150 = base.createAutonomousSystem(150)

# Create a network
as150.createNetwork('net')

# Create a router and connect it to the network
as150.createRouter('router').joinNetwork('net')

# Create three hosts called web and connect them to the network
as150.createHost('web0').joinNetwork('net')
as150.createHost('web1').joinNetwork('net')
as150.createHost('web2').joinNetwork('net')

ppov.bindNode(150, 'web0', 0)
ppov.bindNode(150, 'web1', 1)
ppov.bindNode(150, 'web2', 2)
ppov.bindNode(150, 'router', 3)

###############################################################################
# Rendering
emu.addLayer(base)
emu.addLayer(ppov)
emu.render()

###############################################################################
# Compilation
emu.compile(Docker(), './output', True)
