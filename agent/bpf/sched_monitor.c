// +build ignore

#include <linux/bpf.h>
#include <linux/types.h>
#include <bpf/bpf_helpers.h>

char __license[] SEC("license") = "Dual MIT/GPL";

struct energy_window_t {
    __u32 pid;
    __u32 tgid;
    __u64 on_cpu_ns;
    char  comm[16];
} __attribute__((packed));

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 1 << 16); // 64k entries
} events SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, __u32);
    __type(value, __u64);
    __uint(max_entries, 10240); // Rastrear até 10k threads concorrentes
} start_times SEC(".maps");

// Estrutura do tracepoint sched_switch
struct sched_switch_args {
    unsigned short common_type;
    unsigned char common_flags;
    unsigned char common_preempt_count;
    int common_pid;

    char prev_comm[16];
    int prev_pid;
    int prev_prio;
    long long prev_state;
    char next_comm[16];
    int next_pid;
    int next_prio;
};

SEC("tracepoint/sched/sched_switch")
int handle_sched_switch(struct sched_switch_args *ctx) {
    __u64 now = bpf_ktime_get_ns();
    __u32 prev_pid = ctx->prev_pid;
    __u32 next_pid = ctx->next_pid;

    // 1. Processar a thread que está saindo da CPU (prev_pid)
    __u64 *start_time = bpf_map_lookup_elem(&start_times, &prev_pid);
    if (start_time) {
        __u64 delta = now - *start_time;
        if (delta > 0) {
            struct energy_window_t *event = bpf_ringbuf_reserve(&events, sizeof(*event), 0);
            if (event) {
                __u64 id = bpf_get_current_pid_tgid();
                __u32 current_tid = (__u32)id;
                __u32 current_tgid = id >> 32;

                event->pid = prev_pid;
                if (current_tid == prev_pid) {
                    event->tgid = current_tgid;
                } else {
                    event->tgid = prev_pid; // Fallback se o contexto diferir
                }
                event->on_cpu_ns = delta;

                // Copiar o comando
                #pragma unroll
                for (int i = 0; i < 16; i++) {
                    event->comm[i] = ctx->prev_comm[i];
                }

                bpf_ringbuf_submit(event, 0);
            }
        }
        bpf_map_delete_elem(&start_times, &prev_pid);
    }

    // 2. Registrar o timestamp de entrada da thread que assume a CPU (next_pid)
    bpf_map_update_elem(&start_times, &next_pid, &now, BPF_ANY);

    return 0;
}
