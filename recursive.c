#include <stdio.h>
#include <string.h>
#include <unistd.h>
#include <omp.h>
#include <stdlib.h>
#include <stdbool.h>

int _exception = 0;

#define MAX_STRING_LENGTH 256
typedef struct RecursiveMadness {
    int width;
    int height;
    int current_generation;
    int* grid;
    int* next_grid;
    int depth;
} RecursiveMadness;

RecursiveMadness* RecursiveMadness_new();




RecursiveMadness* RecursiveMadness_new() {
    RecursiveMadness* this = malloc(sizeof(RecursiveMadness));
    this->depth = 0;
    this->depth = 0;
    return this;
}

void RecursiveMadness_start(RecursiveMadness* this, int x);
void RecursiveMadness_foo(RecursiveMadness* this, int level);
void RecursiveMadness_bar(RecursiveMadness* this, int count);
int RecursiveMadness_get_limit(RecursiveMadness* this, int x);

void RecursiveMadness_start(RecursiveMadness* this, int x) {
    printf("Starting madness with %d\n", x);
    RecursiveMadness_foo(this, x);
}

void RecursiveMadness_foo(RecursiveMadness* this, int level) {
    int result = level;
    if (result > 0) {
        int level = result - 1;
        RecursiveMadness_bar(this, level);
    }
    else {
        printf("Reached base in foo\n");
    }
}

void RecursiveMadness_bar(RecursiveMadness* this, int count) {
    for (int i = 0; i <= (this->get_limit(count)); i++) {
        int count = i;
        printf("bar loop i=%d, count=%d\n", i, count);
    }
    RecursiveMadness_foo(this, count - 1);
}

int RecursiveMadness_get_limit(RecursiveMadness* this, int x) {
    if (x < 0) {
        return 0;
    }
    return x % 4 + 2;
}

int main() {
    RecursiveMadness* insane = RecursiveMadness_new();
    RecursiveMadness_start(insane, 6);
    return 0;
}
