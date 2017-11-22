// sigintwrap: wrapper for non-Cygwin executables, capturing Cygwin SIGINTs and
// forwarding them as a CTRL+BREAK events to the non-Cygwin executable. After
// "Solution For Handling Signals In Non-Cygwin Apps With
// SetConsoleCtrlHandler", Anthony DeRosa,
// http://marc.info/?l=cygwin&m=111047278517873


#include <sys/cygwin.h>
#include <pthread.h>
#include <signal.h>
#include <stdio.h>
#include <unistd.h>
#include <windows.h>


static PROCESS_INFORMATION pi;


static void *
wait_for_process(void *ptr) {
  WaitForSingleObject(pi.hProcess, INFINITE);
  return NULL;
}


static void
sigint(int signal) {
  GenerateConsoleCtrlEvent(CTRL_BREAK_EVENT, pi.dwProcessId);
}


static int
needs_path_conversion(const char *s) {
  // See winsup/cygwin/environ.cc.
  return !(strncmp(s, "HOME=", 5) &&
           strncmp(s, "LD_LIBRARY_PATH=", 16) &&
           strncmp(s, "PATH=", 5) &&
           strncmp(s, "TEMP=", 5) &&
           strncmp(s, "TMP=", 4) &&
           strncmp(s, "TMPDIR=", 7));
}


static char *
prepare_env() {
  char **p;

  int len = 1;
  for(p = environ; *p; p++)
    if(needs_path_conversion(*p)) {
      char *eq = strchr(*p, '=');
      len += eq - *p + 1;
      len += cygwin_conv_path_list(CCP_POSIX_TO_WIN_A, eq + 1, NULL, 0);
    } else
      len += strlen(*p) + 1;

  char *env = (char *)malloc(len);
  char *e = env;
  for(p = environ; *p; p++)
    if(needs_path_conversion(*p)) {
      char *eq = strchr(*p, '=');
      e = stpncpy(e, *p, eq - *p + 1);
      cygwin_conv_path_list(CCP_POSIX_TO_WIN_A, eq + 1, e, env + len - e - 1);
      while(*e++);
    } else
      e = stpcpy(e, *p) + 1;
  *e = '\0';

  return env;
}


int
main(int argc, char **argv) {
  if(argc != 2) {
    fprintf(stderr, "usage: %s 'c:\\path\\to\\command.exe [arg...]'\n",
            argv[0]);
    return 1;
  }

  STARTUPINFO si;
  ZeroMemory(&si, sizeof(si));
  si.cb = sizeof(si);

  char *env = prepare_env();

  if(!CreateProcess(NULL, argv[1], NULL, NULL, FALSE, CREATE_NEW_PROCESS_GROUP,
                    env, NULL, &si, &pi)) {
    LPTSTR msg;
    FormatMessage(FORMAT_MESSAGE_ALLOCATE_BUFFER | FORMAT_MESSAGE_FROM_SYSTEM |
                  FORMAT_MESSAGE_IGNORE_INSERTS, NULL, GetLastError(), 0,
                  (LPTSTR)&msg, 0, NULL);
    fputs(msg, stderr);
    LocalFree(msg);
    free(env);
    return 1;
  }

  free(env);

  signal(SIGINT, sigint);

  // We call WaitForSingleObject on another thread because it cannot be
  // interrupted by cygwin signals.  pthread_join can be.
  pthread_t thread;
  pthread_create(&thread, NULL, wait_for_process, NULL);
  pthread_join(thread, NULL);

  DWORD exitcode;
  GetExitCodeProcess(pi.hProcess, &exitcode);

  CloseHandle(pi.hProcess);
  CloseHandle(pi.hThread);

  return exitcode;
}
