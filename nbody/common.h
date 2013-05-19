#ifndef _INCLUDED_COMMON_H
#define _INCLUDED_COMMON_H

typedef enum {
    CAT_UNKNOWN = 0,
    CAT_INSPIRE = 1,
#define CAT(id, str) CAT_##id,
#include "cats.h"
#undef CAT
    CAT_NUMBER_OF
} category_t;

const char *category_enum_to_str(category_t cat);
category_t category_str_to_enum(const char *str);
category_t category_strn_to_enum(const char *str, size_t n);

#define PAPER_MAX_CATS (4)
typedef struct _paper_t {
    // stuff loaded from the DB
    unsigned int id;
    byte allcats[PAPER_MAX_CATS]; // store fixed number of categories; more efficient than having a tiny, dynamic array; unused entries are CAT_UNKNOWN
    short num_refs;
    short num_cites;
    struct _paper_t **refs;
    byte *refs_ref_freq;
    struct _paper_t **cites;
    int index;
    const char *authors;
    const char *title;

    int num_keywords;
    struct _keyword_t **keywords;

    bool pos_valid;
    float x;
    float y;

    // stuff for colouring
    int colour;
    int num_with_my_colour;

    // stuff for connecting disconnected papers
    int num_fake_links;
    struct _paper_t **fake_links;

    // stuff for tred
    int tred_visit_index;
    int *refs_tred_computed;
    struct _paper_t *tred_follow_back_paper;
    int tred_follow_back_ref;

    // stuff for the placement of papers
    bool included;
    int num_included_cites;
    bool connected;
    float age; // between 0.0 and 1.0
    float r;
    float mass;

    struct _layout_node_t *layout_node;
} paper_t;

typedef struct _keyword_t {
    char *keyword;
    int num_papers; // number of papers with this keyword
    float x;
    float y;
} keyword_t;

int date_to_unique_id(int y, int m, int d);
void unique_id_to_date(int id, int *y, int *m, int *d);
void recompute_num_included_cites(int num_papers, paper_t *papers);
void recompute_colours(int num_papers, paper_t *papers, int verbose);
void compute_tred(int num_papers, paper_t *papers);

#endif // _INCLUDED_COMMON_H
