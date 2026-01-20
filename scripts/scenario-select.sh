#!/bin/bash
# Scenario selector for ribbin testing
# Presents a menu of scenarios and runs the selected one

set -e

SCENARIOS_DIR="/usr/local/share/ribbin-scenarios"

# List available scenarios
list_scenarios() {
    echo "Available scenarios:"
    echo ""
    local i=1
    for scenario in "$SCENARIOS_DIR"/*.sh; do
        if [[ -f "$scenario" ]]; then
            name=$(basename "$scenario" .sh)
            # Read description from first comment line
            desc=$(grep -m1 '^# Description:' "$scenario" | sed 's/^# Description: //')
            printf "  %d. %-20s %s\n" "$i" "$name" "$desc"
            ((i++))
        fi
    done
    echo ""
    echo "  q. Quit"
    echo ""
}

# Get scenario by number
get_scenario_by_number() {
    local num=$1
    local i=1
    for scenario in "$SCENARIOS_DIR"/*.sh; do
        if [[ -f "$scenario" ]]; then
            if [[ $i -eq $num ]]; then
                echo "$scenario"
                return
            fi
            ((i++))
        fi
    done
}

# Get scenario by name
get_scenario_by_name() {
    local name=$1
    local scenario="$SCENARIOS_DIR/${name}.sh"
    if [[ -f "$scenario" ]]; then
        echo "$scenario"
    fi
}

# Count scenarios
count_scenarios() {
    local count=0
    for scenario in "$SCENARIOS_DIR"/*.sh; do
        if [[ -f "$scenario" ]]; then
            ((count++))
        fi
    done
    echo "$count"
}

# Main
main() {
    # Check for command line argument
    if [[ -n "$1" ]]; then
        if [[ "$1" == "-h" || "$1" == "--help" ]]; then
            echo "Usage: scenario-select [scenario-name]"
            echo ""
            list_scenarios
            exit 0
        fi

        # Try to find scenario by name
        scenario=$(get_scenario_by_name "$1")
        if [[ -z "$scenario" ]]; then
            echo "Unknown scenario: $1"
            echo ""
            list_scenarios
            exit 1
        fi
    else
        # Interactive selection
        list_scenarios

        local max=$(count_scenarios)
        printf "Enter selection (1-%d or q): " "$max"
        read -r selection

        if [[ "$selection" == "q" || "$selection" == "Q" ]]; then
            echo "Exiting."
            exit 0
        fi

        if ! [[ "$selection" =~ ^[0-9]+$ ]] || [[ "$selection" -lt 1 ]] || [[ "$selection" -gt "$max" ]]; then
            echo "Invalid selection."
            exit 1
        fi

        scenario=$(get_scenario_by_number "$selection")
    fi

    # Run the selected scenario
    echo ""
    echo "Running scenario: $(basename "$scenario" .sh)"
    echo ""

    # Source the scenario to run it
    source "$scenario"
}

main "$@"
