#!/usr/bin/env bash
set -xe
loop=0
command="make clean -C /opt/metal3-dev-env/metal3-dev-env/ && sudo rm -rf /opt/metal3-dev-env/ && make test-e2e"
# command="echo hello && echo hi"
while true; do
  loop=$((loop+1))
 eval "$command" | tee -a stdout
 if grep "STOPSTOPSTOP" stdout ; then
   echo "Failed at the $loop time" | tee -a stdout
   exit 1
 else 
   echo "Pass the $loop time" | tee -a stdout
 fi
done
