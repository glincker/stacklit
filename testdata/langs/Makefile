include config.mk
-include local.mk

CC := gcc

.PHONY: all clean

all: app

app: main.o util.o
	$(CC) -o $@ $^

%.o: %.c
	$(CC) -c $<

clean:
	rm -f *.o app
