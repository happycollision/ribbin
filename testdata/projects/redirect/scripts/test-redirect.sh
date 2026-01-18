#!/bin/bash
# Test redirect script for integration tests
# This script is invoked when the redirect action is triggered
# It echoes environment variables set by ribbin for verification

echo "REDIRECT_CALLED=true"
echo "RIBBIN_ORIGINAL_BIN=$RIBBIN_ORIGINAL_BIN"
echo "RIBBIN_COMMAND=$RIBBIN_COMMAND"
echo "RIBBIN_CONFIG=$RIBBIN_CONFIG"
echo "RIBBIN_ACTION=$RIBBIN_ACTION"
echo "ARGS=$@"
exit 0
