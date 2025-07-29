#define len(x) (sizeof(x) / sizeof((x)[0]))
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

typedef struct math_Calculator
{
} math_Calculator;

math_Calculator *math_Calculator_new();

extern int math_PI;
int math_PI = 3;
math_Calculator *math_Calculator_new()
{
    math_Calculator *obj = malloc(sizeof(math_Calculator));
    return obj;
}

int math_Calculator_add(math_Calculator *this, int a, int b)
{
    return a + b;
}

int main()
{
    int area = math.PI * 5 * 5;
    Calculator *calc = Calculator_new();
    int result = Calculator_add(calc, 10, 20);
    return 0;
}
