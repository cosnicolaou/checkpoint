#!/bin/bash

eval $(checkpoint use $(basename $0))
echo $CHECKPOINT_SESSION_ID
