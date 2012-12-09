#ifndef _INCLUDED_MAP_H
#define _INCLUDED_MAP_H

typedef struct _map_env_t map_env_t;

map_env_t *map_env_new();
void map_env_set_papers(map_env_t *map_env, int num_papers, paper_t *papers);
void map_env_random_papers(map_env_t *map_env, int n);
void map_env_papers_test1(map_env_t *map_env, int n);
void map_env_draw(map_env_t *map_env, cairo_t *cr, guint width, guint height);
bool map_env_forces(map_env_t *map_env, bool do_attr);
void map_env_grow(map_env_t *map_env, double amt);

#endif // _INCLUDED_MAP_H
