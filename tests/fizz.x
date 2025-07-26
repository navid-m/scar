print "Starting FizzBuzz..."
sleep 1

for i = 1 to 100:
    if i % 3 == 0 && i % 5 == 0:
        print "FizzBuzz"
    elif i % 3 == 0:
        print "Fizz"
    elif i % 5 == 0:
        print "Buzz"
    else:
        print "%d" | i
    sleep 0.1
    
print "FizzBuzz complete!"
