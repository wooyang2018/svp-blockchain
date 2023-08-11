#!/bin/bash

########## Change to the Path where the Script Is Located ##########
script_path="$(cd "$(dirname "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
cd "$script_path"

########## Some Functions for Modifying Global Variables ##########
#consensus/config.go#TwoPhaseBFTFlag
function set_two_phase_bft_flag() {
  echo "set_two_phase_bft_flag $1"
  sed -E -i 's/TwoPhaseBFTFlag\s*=\s*(true|false)/TwoPhaseBFTFlag='$1'/' ../consensus/config.go
}
#consensus/config.go#VoteStrategy
function set_vote_strategy() {
  echo "set_vote_strategy $1"
  sed -E -i 's/VoteStrategy\s*:\s*[A-Za-z]+/VoteStrategy:'$1'/' ../consensus/config.go
}
#tests/main.go#NodeCount
function set_node_count() {
  echo "set_node_count $1"
  sed -E -i 's/NodeCount\s*=\s*[0-9]+/NodeCount='$1'/' ./main.go
}
#tests/main.go#WindowSize
function set_window_size() {
  echo "set_window_size $1"
  sed -E -i 's/\tWindowSize\s*=\s*[0-9]+/\tWindowSize='$1'/' ./main.go
}

########## Preparation for All Experiments ##########
mkdir -p ./workdir
rm -rf ./workdir/benchmarks/
>./workdir/experiment-posv.log
>./workdir/experiment-bft.log

########## Experiment 1: Basic Performance ##########
set_two_phase_bft_flag false
set_vote_strategy "AverageVote"
set_window_size 4
echo "> starting experiment 1: PoSV"
for i in {4..8..4}; do
  set_node_count "$i"
  go run . >>./workdir/experiment-posv.log 2>&1
done
mv ./workdir/benchmarks/ ./workdir/experiment-posv/
echo -e "> finished experiment 1: PoSV\n"

set_two_phase_bft_flag true
set_window_size 1
echo "> starting experiment 1: Two Phase BFT"
for i in {4..8..4}; do
  set_node_count "$i"
  go run . >>./workdir/experiment-bft.log 2>&1
done
mv ./workdir/benchmarks/ ./workdir/experiment-bft/
echo -e "> finished experiment 1: Two Phase BFT\n"
