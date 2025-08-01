int n = 5
int fact = 1

print "Calculating factorial of %d..." | n

for i = 1 to n:
    reassign fact = fact * i
    print "i = %d, fact = %d" | i, fact
    sleep 0.2

print "Final result: %d! = %d" | n, fact
