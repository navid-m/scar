int n = 29
bool is_prime = 1

print "Checking if %d is prime..." | n

if n <= 1:
    is_prime = 0
else:
    for i = 2 to (n - 1):
        if n % i == 0:
            is_prime = 0
            break

if is_prime:
    print "%d is a prime number." | n
else:
    print "%d is NOT a prime number." | n
