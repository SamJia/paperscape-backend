#ifndef _INCLUDED_MYSQL_H
#define _INCLUDED_MYSQL_H

#include "util/hashmap.h"
#include "common.h"
#include "category.h"
#include "layout.h"

bool mysql_load_papers(const char *where_clause, bool load_authors_and_titles, category_set_t *category_set, int *num_papers_out, paper_t **papers_out, hashmap_t **keyword_set_out);

// Used by Mapmysql.c
bool mysql_save_paper_positions(layout_t *layout);
bool mysql_load_paper_positions(layout_t *layout);

#endif // _INCLUDED_MYSQL_H 
