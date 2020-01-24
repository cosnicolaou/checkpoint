#!/bin/zsh

proposed="date-for-example"

source <(checkpoint use $(basename $proposed))
trap completed EXIT

if completed ready; then
  echo "already processed"
  exit 0
fi
echo "new data"
exit 0
