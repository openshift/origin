#!/bin/bash
set -e

function dynamic_cpu_sizing {
    total_cpu=$1
    # Base allocation for 1 CPU in fractions of a core (60 millicores = 0.06 CPU core)
    base_allocation_fraction=0.06
    # Increment per additional CPU in fractions of a core (12 millicores = 0.012 CPU core)
    increment_per_cpu_fraction=0.012
    if ((total_cpu > 1)); then
        # Calculate the total system-reserved CPU in fractions, starting with the base allocation
        # and adding the incremental fraction for each additional CPU
        recommended_systemreserved_cpu=$(awk -v base="$base_allocation_fraction" -v increment="$increment_per_cpu_fraction" -v cpus="$total_cpu" 'BEGIN {printf "%.2f\n", base + increment * (cpus - 1)}')
    else
        # For a single CPU, use the base allocation
        recommended_systemreserved_cpu=$base_allocation_fraction
    fi

    # Enforce minimum threshold of 0.5 CPU
    recommended_systemreserved_cpu=$(awk -v val="$recommended_systemreserved_cpu" 'BEGIN {if (val < 0.5) print 0.5; else print val}')

    echo "SYSTEM_RESERVED_CPU=${recommended_systemreserved_cpu}"
}

dynamic_cpu_sizing $1