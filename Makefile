CC = gcc
CFLAGS = -Wall -ansi -std=gnu99 -O3 -I../clib $(shell pkg-config --cflags gtk+-3.0)
LDFLAGS = $(shell pkg-config --libs gtk+-3.0)

SRC = \
	common.c \
	tred.c \
	quadtree.c \
	map.c \
	mysql.c \
	gen.c \
	gui.c \
	cairohelper.c \
	tile.c \
	main.c \

OBJ = $(SRC:.c=.o)
LIB = -lm -lmysqlclient ../clib/xiwilib.a
PROG = mapgen

$(PROG): $(OBJ)
	$(CC) $(LDFLAGS) -o $@ $(OBJ) $(LIB)

clean:
	/bin/rm $(OBJ)

depend:
	makedepend -Y $(SRC) 2>/dev/null

