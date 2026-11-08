#include <stdio.h>
#include "util.h"

typedef struct Node { int val; struct Node *next; } Node;

struct Config { int port; };

enum Color { RED, GREEN };

static int helper(int a) { return a; }

int add(int a, int b) {
    return a + b;
}

void run(const char *name);
