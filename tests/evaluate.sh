#!/bin/bash

########## Change to the Path where the Script Is Located ##########
script_path="$(cd "$(dirname "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
cd "$script_path"
rm -rf ./workdir/benchmarks/

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

########## Experiment 1: Basic Performance ##########
function run_experiment_basic() {
  mkdir -p ./workdir/experiment-posv/
  mkdir -p ./workdir/experiment-bft/
  >./workdir/experiment-posv.log
  >./workdir/experiment-bft.log

  set_two_phase_bft_flag false
  set_vote_strategy "AverageVote"
  set_window_size 4
  echo "> starting experiment 1: PoSV"
  for i in {4..28..4}; do
    set_node_count "$i"
    go run . >>./workdir/experiment-posv.log 2>&1
  done
  mv ./workdir/benchmarks/* ./workdir/experiment-posv/
  echo -e "> finished experiment 1: PoSV\n"

  set_two_phase_bft_flag true
  set_window_size 1
  echo "> starting experiment 1: Two Phase BFT"
  for i in {4..28..4}; do
    set_node_count "$i"
    go run . >>./workdir/experiment-bft.log 2>&1
  done
  mv ./workdir/benchmarks/* ./workdir/experiment-bft/
  echo -e "> finished experiment 1: Two Phase BFT\n"
}
run_experiment_basic

########## Experiment 2: Impact of Voting Window ##########
function run_experiment_window() {
  mkdir -p ./workdir/experiment-average/
  mkdir -p ./workdir/experiment-random/
  >./workdir/experiment-average.log
  >./workdir/experiment-random.log
  set_two_phase_bft_flag false
  set_node_count 28

  set_vote_strategy "AverageVote"
  echo "> starting experiment 2: AverageVote"
  for i in {4..19..3}; do
    set_window_size "$i"
    go run . >>./workdir/experiment-average.log 2>&1
  done
  mv ./workdir/benchmarks/* ./workdir/experiment-average/
  echo -e "> finished experiment 2: AverageVote\n"

  set_vote_strategy "RandomVote"
  echo "> starting experiment 2: RandomVote"
  for i in {4..19..3}; do
    set_window_size "$i"
    go run . >>./workdir/experiment-random.log 2>&1
  done
  mv ./workdir/benchmarks/* ./workdir/experiment-random/
  echo -e "> finished experiment 2: RandomVote\n"
}
run_experiment_window

########## Experiment 3 and 4: Security in Two Attack Scenarios ##########
function run_experiment_security() {
  mkdir -p ./workdir/experiment-ordinary/
  mkdir -p ./workdir/experiment-monopoly/
  >./workdir/experiment-security.log
  set_two_phase_bft_flag false
  set_node_count 28
  set_window_size 4

  set_vote_strategy "OrdinaryVote"
  go run . >>./workdir/experiment-security.log 2>&1
  mv ./workdir/benchmarks/* ./workdir/experiment-ordinary/

  set_vote_strategy "MonopolyVote"
  go run . >>./workdir/experiment-security.log 2>&1
  mv ./workdir/benchmarks/* ./workdir/experiment-monopoly/
}
run_experiment_security
