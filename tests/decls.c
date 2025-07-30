#define len(x) (sizeof(x) / sizeof((x)[0]))
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>


int main() {
    int x = 5;
    printf("%d hello\n", x);
    return 0;
}
