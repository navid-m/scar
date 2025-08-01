# Reproducing unknown_0 bug.

pub fn res() -> float:
    float result = 0.0;
    while result < 100.0:
        reassign result = result + 1.0
    return result

print "Result: %f" | res()