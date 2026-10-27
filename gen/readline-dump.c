/* readline-dump prints the defaults of the GNU readline library it is linked
   against, one fact to a line, for gen.go to turn into bind.go.

     readline <version>
     command <name>
     var <name> <value>
     bind <keymap> <command> <key sequence as an inputrc file writes it>

   It uses the public API only: rl_funmap_names, rl_invoking_keyseqs_in_map
   and rl_variable_dumper. Run it with INPUTRC=/dev/null in a UTF-8 locale,
   because readline sets the four meta variables from the locale.

     cc -o readline-dump readline-dump.c -lreadline */
#include <locale.h>
#include <stdio.h>
#include <stdlib.h>
#include <readline/readline.h>

static const char *keymaps[] = {
  "emacs", "emacs-standard", "emacs-meta", "emacs-ctlx",
  "vi", "vi-move", "vi-command", "vi-insert", NULL,
};

int main(void) {
  setlocale(LC_ALL, "");
  setenv("INPUTRC", "/dev/null", 1);
  rl_readline_name = "inputrc-gen";
  rl_outstream = stdout;
  rl_catch_signals = 0;
  rl_catch_sigwinch = 0;
  rl_initialize();
  printf("readline %s\n", rl_library_version);
  const char **names = rl_funmap_names();
  for (int i = 0; names != NULL && names[i] != NULL; i++) printf("command %s\n", names[i]);
  /* rl_variable_dumper in its readable form writes "set <name> <value>". */
  fflush(stdout);
  rl_variable_dumper(1);
  fflush(stdout);
  for (int k = 0; keymaps[k] != NULL; k++) {
    Keymap map = rl_get_keymap_by_name(keymaps[k]);
    for (int i = 0; names != NULL && names[i] != NULL; i++) {
      char **seqs = rl_invoking_keyseqs_in_map(rl_named_function(names[i]), map);
      if (seqs == NULL) {
        printf("unbound %s %s\n", keymaps[k], names[i]);
        continue;
      }
      for (int j = 0; seqs[j] != NULL; j++) {
        printf("bind %s %s %s\n", keymaps[k], names[i], seqs[j]);
        free(seqs[j]);
      }
      free(seqs);
    }
  }
  free(names);
  return 0;
}
