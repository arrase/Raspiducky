#include <unistd.h>
#include <stdlib.h>
int main(int argc, char **argv) {
    usleep( atol( argv[1] ) );
}
