#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/time.h>

#include "util/xiwilib.h"
#include "common.h"
#include "layout.h"
#include "map.h"
#include "mapmysql.h"
#include "mapauto.h"
#include "mysql.h"
#include "json.h"

static int usage(const char *progname) {
    printf("\n");
    printf("usage: %s [options]\n", progname);
    printf("\n");
    printf("options:\n");
    printf("    --settings, -s <file>     load settings from given JSON file\n");
    printf("    --start-afresh            start the graph layout afresh (default is to process\n");
    printf("                              only new papers); enabling this enables --write-json\n");
    printf("    --layout-json <file>      load layout from given JSON file (default is from DB)\n");
    printf("    --refs-json <file>        load reference data from JSON file (default is from DB)\n");
    printf("    --other-links <file>      load additional links from JSON file\n");
    printf("    --whole-arxiv             process all papers from the arxiv (default is to\n");
    printf("                              process only a small, test subset)\n");
    printf("    --write-db                write positions to DB (default is not to)\n");
    printf("    --write-json              write positions to json file (default is not to)\n");
    printf("    --no-fake-links, -nf      don't create fake links; --start-afresh must also be set\n");
    printf("    --link <num>              link strength\n");
    printf("    --rsq <num>               r-star squared distance for anti-gravity\n");
    printf("    --factor-ref-link <num>   factor to use for reference links (default 1)\n");
    printf("    --factor-other-link <num> factor to use for other links (default 0)\n");
    printf("\n");
    return 1;
}

int main(int argc, char *argv[]) {

    // parse command line arguments
    bool arg_start_afresh = false;
    //const char *where_clause = "(arxiv IS NOT NULL AND status != 'WDN' AND id > 2130000000 AND maincat='hep-th')";
    bool arg_write_db            = false;
    bool arg_write_json          = false;
    bool arg_no_fake_links       = false;
    double arg_anti_grav_rsq     = -1;
    double arg_link_strength     = -1;
    const char *arg_settings     = NULL;
    const char *arg_layout_json  = NULL;
    const char *arg_refs_json    = NULL;
    const char *arg_other_links  = NULL;
    double arg_factor_ref_link   = 1;
    double arg_factor_other_link = 0;
    for (int a = 1; a < argc; a++) {
        if (streq(argv[a], "--settings") || streq(argv[a], "-s")) {
            a += 1;
            if (a >= argc) {
                return usage(argv[0]);
            }
            arg_settings = argv[a];
        } else if (streq(argv[a], "--start-afresh")) {
            arg_start_afresh = true;
            arg_write_json = true;
        } else if (streq(argv[a], "--layout-json")) {
            a += 1;
            if (a >= argc) {
                return usage(argv[0]);
            }
            arg_layout_json = argv[a];
        } else if (streq(argv[a], "--refs-json")) {
            a += 1;
            if (a >= argc) {
                return usage(argv[0]);
            }
            arg_refs_json = argv[a];
        } else if (streq(argv[a], "--other-links")) {
            if (++a >= argc) {
                return usage(argv[0]);
            }
            arg_other_links = argv[a];
        } else if (streq(argv[a], "--whole-arxiv")) {
            // TODO remove this flag from autoupdate scripts
            //where_clause = "(arxiv IS NOT NULL AND status != 'WDN')";
        } else if (streq(argv[a], "--write-db")) {
            arg_write_db = true;
        } else if (streq(argv[a], "--write-json")) {
            arg_write_json = true;
        } else if (streq(argv[a], "--rsq")) {
            a += 1;
            if (a >= argc) {
                return usage(argv[0]);
            }
            arg_anti_grav_rsq = strtod(argv[a], NULL);
        } else if (streq(argv[a], "--link")) {
            a += 1;
            if (a >= argc) {
                return usage(argv[0]);
            }
            arg_link_strength = strtod(argv[a], NULL);
        } else if (streq(argv[a], "--no-fake-links") || streq(argv[a], "-nf")) {
            arg_no_fake_links = true;
        } else if (streq(argv[a], "--factor-ref-link") || streq(argv[a], "-fr")) {
            if (++a >= argc) {
                return usage(argv[0]);
            }
            arg_factor_ref_link = strtod(argv[a], NULL);;
        } else if (streq(argv[a], "--factor-other-link") || streq(argv[a], "-fo")) {
            if (++a >= argc) {
                return usage(argv[0]);
            }
            arg_factor_other_link = strtod(argv[a], NULL);;
        } else {
            return usage(argv[0]);
        }
    }

    // load settings from json file
    const char *settings_file = "settings/default.json";
    if (arg_settings != NULL) {
        settings_file = arg_settings;
    }
    init_config_t *init_config = init_config_new(settings_file);
    if (init_config == NULL) {
        return 1;
    }

    // print info about the where clause being used
    printf("using where clause: %s\n", init_config->sql_extra_clause);

    int num_papers;
    paper_t *papers;
    hashmap_t *keyword_set;
    if (arg_refs_json == NULL) {
        // load the papers from the DB
        if (!mysql_load_papers(init_config, false, &num_papers, &papers, &keyword_set)) {
            return 1;
        }
    } else {
        // load the papers from JSON file
        if (!json_load_papers(arg_refs_json, &num_papers, &papers, &keyword_set)) {
            return 1;
        }
    }
    if (arg_start_afresh && arg_other_links != NULL) {
        // f starting afresh, allow loading other links from JSON file
        if (!json_load_other_links(arg_other_links, num_papers, papers)) {
            return 1;
        }
    }

    // create the map object
    map_env_t *map_env = map_env_new();

    // set initial configuration
    map_env_set_init_config(map_env,init_config);
    
    if (arg_start_afresh && arg_no_fake_links) {
        // if starting afresh, allow user to disable fake link generation
        map_env_set_make_fake_links(map_env,!arg_no_fake_links);
    }

    // set parameters
    if (arg_anti_grav_rsq > 0) {
        map_env_set_anti_gravity(map_env, arg_anti_grav_rsq);
    }
    if (arg_link_strength > 0) {
        map_env_set_link_strength(map_env, arg_link_strength);
    }
    printf("using a link strength of: %.3f\n",map_env_get_link_strength(map_env));

    // set the papers
    map_env_set_papers(map_env, num_papers, papers, keyword_set);

    // select the date range
    unsigned int id_min;
    unsigned int id_max;
    map_env_get_max_id_range(map_env, &id_min, &id_max);
    map_env_select_date_range(map_env, id_min, id_max);

    if (arg_start_afresh) {
        // create a new layout with 10 levels of coarsening
        map_env_layout_new(map_env, 10, arg_factor_ref_link, arg_factor_other_link);
        // do the layout
        map_env_do_complete_layout(map_env, 2000, 6000);
    } else {
        if (arg_layout_json == NULL) {
            // load existing positions from DB
            map_env_layout_pos_load_from_db(map_env);
        } else {
            // load existing positions from json file
            map_env_layout_pos_load_from_json(map_env, arg_layout_json);
        }

        // rotate the entire map by a random amount, to reduce quad-tree-force artifacts
        struct timeval tp;
        gettimeofday(&tp, NULL);
        srandom(tp.tv_sec * 1000000 + tp.tv_usec);
        double angle = 6.28 * (double)random() / (double)RAND_MAX;
        map_env_rotate_all(map_env, angle);
        printf("rotated graph by %.2f rad to eliminate quad-tree-force artifacts\n", angle);

        // assign positions to new papers
        int n_new = map_env_layout_place_new_papers(map_env);
        if (n_new > 0) {
            printf("iterating to place new papers\n");
            map_env_set_do_close_repulsion(map_env, false);
            map_env_do_iterations(map_env, 250, false, false);
        }
        map_env_layout_finish_placing_new_papers(map_env);

        // iterate to adjust whole graph
        printf("iterating to adjust entire graph\n");
        map_env_set_do_close_repulsion(map_env, true);
        map_env_do_iterations(map_env, 80, false, false);
    
        // iterate for final, very fine steps
        printf("iterating final, very fine steps\n");
        map_env_set_do_close_repulsion(map_env, true);
        map_env_do_iterations(map_env, 30, false, true);
    }

    // align the map in a fixed direction
    if (!arg_start_afresh) {
        // TODO currently hardcoded for Paperscape
        map_env_orient_using_category(map_env, CAT_hep_ph, 4.2);
    } else if (num_papers > 0) {
        map_env_orient_using_paper(map_env, &papers[0], 0);
    }

    // write the new positions to the DB (never do this for timelapse)
    if (arg_write_db) {
        map_env_layout_pos_save_to_db(map_env);
    }

    // write map to JSON (always do this for timelapse)
    if (arg_write_json) {
        vstr_t *vstr = vstr_new();
        vstr_reset(vstr);
        vstr_printf(vstr, "map-%06u.json", map_env_get_num_papers(map_env));
        map_env_layout_pos_save_to_json(map_env, vstr_str(vstr));
    }

    return 0;
}
