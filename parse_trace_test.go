package phonelab

import (
	"testing"
	//	"github.com/stretchr/testify/assert"
)

// TODO: test trace tags
/*

	str := "aeea32238ddb516568b10685a5f38089a6450252        1462470176659   1462470176659.25        29b2b79e-1a97-4f96-8070-7a26f952e92b    31950   1932.849444     2016-05-05 17:42:56.659837      216     216     D	       Kernel-Trace    kworker/0:3-2658  [000] ...1  1932.849097: sched_cpu_hotplug: cpu 1 online error=0"

	logline, err := ParseLogline(str)
	assert.NotNil(logline)
	assert.Nil(err)
	trace := ParseTraceFromLoglinePayload(logline)

	assert.NotEqual(nil, trace, "Parsing failed")
	assert.Equal("sched_cpu_hotplug", trace.Tag(), "Tag does not match")
*/

func TestKernelTraceParser(t *testing.T) {
	t.Parallel()

	parser := NewKernelTraceParser()

	testConf := []*parseComparison{
		// Take a proper line and change the tag to something that will not be seen
		&parseComparison{
			line:          "1b0676e5fb2d7ab82a2b76887c53e94cf0410826        1461715200524   1461715200524.17        346fb177-c54f-4f8a-9385-124c461fd5cc    1268385 20456.226252    2016-04-27 00:00:00.524332      203     203     D   Kernel-Trace     kworker/0:2-1911  [000] ...1 20455.979145: thermal_tempERATUR!#$@#: sensor_id=5 temp=32",
			parser:        parser,
			subParseFails: true,
		},
		// Mess around with the trace payload some more
		&parseComparison{
			line:          "1b0676e5fb2d7ab82a2b76887c53e94cf0410826        1461715200524   1461715200524.17        346fb177-c54f-4f8a-9385-124c461fd5cc    1268385 20456.226252    2016-04-27 00:00:00.524332      203     203     D   Kernel-Trace     kworker/0:2-1911  [00a] ...1 20455.979145: thermal_temp: sensor_id=5 temp=32",
			parser:        parser,
			subParseFails: true,
		},
		// And some more...
		&parseComparison{
			line:          "1b0676e5fb2d7ab82a2b76887c53e94cf0410826        1461715200524   1461715200524.17        346fb177-c54f-4f8a-9385-124c461fd5cc    1268385 20456.226252    2016-04-27 00:00:00.524332      203     203     D   Kernel-Trace     kworker/0:2-1911  [000] 20455.979145: thermal_temp: sensor_id=5 temp=32",
			parser:        parser,
			subParseFails: true,
		},
	}

	commonTestParse(testConf, t)
}

/*
func TestCommonParse(t *testing.T) {
	assert := assert.New(t)

	str := "aeea32238ddb516568b10685a5f38089a6450252        1462470176659   1462470176659.25        29b2b79e-1a97-4f96-8070-7a26f952e92b    31950   1932.849444     2016-05-05 17:42:56.659837      216     216     D	       Kernel-Trace    kworker/0:3-2658  [000] ...1  1932.849097: sched_cpu_hotplug: cpu A online error=0"

	// The idea with this test is to get coverage over common errors that this function can throw
	// the per-trace error checking is done in each trace's corresponding test
	trace := NewTrace()
ti := common_parse(str, SCHED_CPU_HOTPLUG_CONST, trace)
	assert.Nil(ti, "Correctly parsed bad trace line")

}
*/

func TestParseSchedCpuHotplug(t *testing.T) {
	t.Parallel()

	parser := NewKernelTraceParser()

	testConf := []*parseComparison{
		// OK
		&parseComparison{
			line: "aeea32238ddb516568b10685a5f38089a6450252        1462470176659   1462470176659.25        29b2b79e-1a97-4f96-8070-7a26f952e92b    31950   1932.849444     2016-05-05 17:42:56.659837      216     216     D	       Kernel-Trace    kworker/0:3-2658  [000] ...1  1932.849097: sched_cpu_hotplug: cpu 1 online error=0",
			parser:        parser,
			subParseFails: false,
			expected:      &SchedCpuHotplug{Cpu: 1, State: "online", Error: 0},
		},
		// Bad cpu
		&parseComparison{
			line: "aeea32238ddb516568b10685a5f38089a6450252        1462470176659   1462470176659.25        29b2b79e-1a97-4f96-8070-7a26f952e92b    31950   1932.849444     2016-05-05 17:42:56.659837      216     216     D	       Kernel-Trace    kworker/0:3-2658  [000] ...1  1932.849097: sched_cpu_hotplug: cpu 19999299009299 online error=0",
			parser:        parser,
			subParseFails: true,
		},
		// Bad error
		&parseComparison{
			line: "aeea32238ddb516568b10685a5f38089a6450252        1462470176659   1462470176659.25        29b2b79e-1a97-4f96-8070-7a26f952e92b    31950   1932.849444     2016-05-05 17:42:56.659837      216     216     D	       Kernel-Trace    kworker/0:3-2658  [000] ...1  1932.849097: sched_cpu_hotplug: cpu 1 online error=281283882183800",
			parser:        parser,
			subParseFails: true,
		},
	}

	commonTestParse(testConf, t)
}

func TestParseThermalTemp(t *testing.T) {
	t.Parallel()

	parser := NewKernelTraceParser()

	testConf := []*parseComparison{
		// Valid
		&parseComparison{
			line:     "1b0676e5fb2d7ab82a2b76887c53e94cf0410826        1461715200524   1461715200524.17        346fb177-c54f-4f8a-9385-124c461fd5cc    1268385 20456.226252    2016-04-27 00:00:00.524332      203     203     D   Kernel-Trace     kworker/0:2-1911  [000] ...1 20455.979145: thermal_temp: sensor_id=5 temp=32",
			parser:   parser,
			expected: &ThermalTemp{Temp: 32, SensorId: 5},
		},
		// Invalid
		&parseComparison{
			line:          "1b0676e5fb2d7ab82a2b76887c53e94cf0410826        1461715200524   1461715200524.17        346fb177-c54f-4f8a-9385-124c461fd5cc    1268385 20456.226252    2016-04-27 00:00:00.524332      203     203     D   Kernel-Trace     kworker/0:2-1911  [000] ...1 20455.979145: thermal_temp: sensor_id=50042002040204 temp=32",
			parser:        parser,
			subParseFails: true,
		},
		// Invalid
		&parseComparison{
			line:          "1b0676e5fb2d7ab82a2b76887c53e94cf0410826        1461715200524   1461715200524.17        346fb177-c54f-4f8a-9385-124c461fd5cc    1268385 20456.226252    2016-04-27 00:00:00.524332      203     203     D   Kernel-Trace     kworker/0:2-1911  [000] ...1 20455.979145: thermal_temp: sensor_id=5 temp=32699699692939495",
			parser:        parser,
			subParseFails: true,
		},
	}

	commonTestParse(testConf, t)
}

func TestParseCpuFrequency(t *testing.T) {
	t.Parallel()

	parser := NewKernelTraceParser()

	testConf := []*parseComparison{
		&parseComparison{
			line:     "aeea32238ddb516568b10685a5f38089a6450252        1462470077472   1462470077472.3 29b2b79e-1a97-4f96-8070-7a26f952e92b    14699   1833.830726     2016-05-05 17:41:17.472999      216     216     D       Kernel-Trace    kworker/0:1H-17    [000] ...1  1833.830633: cpu_frequency: state=1728000 cpu_id=0",
			parser:   parser,
			expected: &CpuFrequency{State: 1728000, CpuId: 0},
		},
		&parseComparison{
			line:     "aeea32238ddb516568b10685a5f38089a6450252        1462470077472   1462470077472.3 29b2b79e-1a97-4f96-8070-7a26f952e92b    14699   1833.830726     2016-05-05 17:41:17.472999      216     216     D       Kernel-Trace    kworker/0:1H-17    [000] ...1  1833.830633: cpu_frequency: state=500000 cpu_id=3",
			parser:   parser,
			expected: &CpuFrequency{State: 500000, CpuId: 3},
		},
		&parseComparison{
			line:          "aeea32238ddb516568b10685a5f38089a6450252        1462470077472   1462470077472.3 29b2b79e-1a97-4f96-8070-7a26f952e92b    14699   1833.830726     2016-05-05 17:41:17.472999      216     216     D       Kernel-Trace    kworker/0:1H-17    [000] ...1  1833.830633: cpu_frequency: state=1728000000000 cpu_id=0",
			parser:        parser,
			subParseFails: true,
		},
		&parseComparison{
			line:          "aeea32238ddb516568b10685a5f38089a6450252        1462470077472   1462470077472.3 29b2b79e-1a97-4f96-8070-7a26f952e92b    14699   1833.830726     2016-05-05 17:41:17.472999      216     216     D       Kernel-Trace    kworker/0:1H-17    [000] ...1  1833.830633: cpu_frequency: state=1728000 cpu_id=993958293994529",
			parser:        parser,
			subParseFails: true,
		},
	}

	commonTestParse(testConf, t)

}

func TestParsePhonelabNumOnlineCpus(t *testing.T) {
	t.Parallel()

	parser := NewKernelTraceParser()

	testConf := []*parseComparison{
		&parseComparison{
			line:     "aeea32238ddb516568b10685a5f38089a6450252        1462470176659   1462470176659.26        29b2b79e-1a97-4f96-8070-7a26f952e92b    31951   1932.849450     2016-05-05 17:42:56.659837      216     216     D       Kernel-Trace    kworker/0:3-2658  [000] ...1  1932.849100: phonelab_num_online_cpus: num_online_cpus=2",
			expected: &PhonelabNumOnlineCpus{NumOnlineCpus: 2},
			parser:   parser,
		},
		&parseComparison{
			line:          "aeea32238ddb516568b10685a5f38089a6450252        1462470176659   1462470176659.26        29b2b79e-1a97-4f96-8070-7a26f952e92b    31951   1932.849450     2016-05-05 17:42:56.659837      216     216     D       Kernel-Trace    kworker/0:3-2658  [000] ...1  1932.849100: phonelab_num_online_cpus: num_online_cpus=29994292949249",
			subParseFails: true,
			parser:        parser,
		},
	}
	commonTestParse(testConf, t)
}

func TestParsePhonelabProcForeground(t *testing.T) {
	t.Parallel()

	parser := NewKernelTraceParser()

	testConf := []*parseComparison{
		&parseComparison{
			line: "0cd58475d61451bd05e96e46c94c9a099dd66ed1        1462418399953   1462418399953.0 00ff336a-b8c2-4641-8276-803fef28dfcb    24157618        115721.275116   2016-05-05 03:19:59.953591      204     204     D	       Kernel-Trace    ndroid.systemui-894   [000] ...1 115721.275037: phonelab_proc_foreground: pid=894 tgid=894 comm=ndroid.systemui",
			parser:   parser,
			expected: &PhonelabProcForeground{Pid: 894, Tgid: 894, Comm: "ndroid.systemui"},
		},
		&parseComparison{
			line: "0cd58475d61451bd05e96e46c94c9a099dd66ed1        1462418399953   1462418399953.0 00ff336a-b8c2-4641-8276-803fef28dfcb    24157618        115721.275116   2016-05-05 03:19:59.953591      204     204     D	       Kernel-Trace    ndroid.systemui-894   [000] ...1 115721.275037: phonelab_proc_foreground: pid=894 tgid=100 comm=ndroid.systemui",
			parser:   parser,
			expected: &PhonelabProcForeground{Pid: 894, Tgid: 100, Comm: "ndroid.systemui"},
		},
		&parseComparison{
			line: "0cd58475d61451bd05e96e46c94c9a099dd66ed1        1462418399953   1462418399953.0 00ff336a-b8c2-4641-8276-803fef28dfcb    24157618        115721.275116   2016-05-05 03:19:59.953591      204     204     D	       Kernel-Trace    ndroid.systemui-894   [000] ...1 115721.275037: phonelab_proc_foreground: pid=89402104020400 tgid=894 comm=ndroid.systemui",
			parser:        parser,
			subParseFails: true,
		},
		&parseComparison{
			line: "0cd58475d61451bd05e96e46c94c9a099dd66ed1        1462418399953   1462418399953.0 00ff336a-b8c2-4641-8276-803fef28dfcb    24157618        115721.275116   2016-05-05 03:19:59.953591      204     204     D	       Kernel-Trace    ndroid.systemui-894   [000] ...1 115721.275037: phonelab_proc_foreground: pid=894 tgid=8949492499249 comm=ndroid.systemui",
			parser:        parser,
			subParseFails: true,
		},
	}
	commonTestParse(testConf, t)
}

func TestParsePhonelabPeriodicCtxSwitchInfo(t *testing.T) {
	t.Parallel()

	parser := NewKernelTraceParser()

	testConf := []*parseComparison{
		&parseComparison{
			line:   "e7a59f87-838c-48a0-bb82-bf97f5e8c79d 2016-07-05 14:19:10.239999912 82421087 [939984.528178]   201   201 D Kernel-Trace:      kworker/1:1-17    [001] ...2 939984.527405: phonelab_periodic_ctx_switch_info: cpu=1 pid=1563 tgid=1561 nice=0 comm=su_daemon utime=0 stime=0 rtime=318437 bg_utime=0 bg_stime=0 bg_rtime=0 s_run=0 s_int=2 s_unint=0 s_oth=0 log_idx=939983 rx=0 tx=0",
			parser: parser,
			expected: &PhonelabPeriodicCtxSwitchInfo{
				Cpu: 1, Pid: 1563, Tgid: 1561, Nice: 0, Comm: "su_daemon", Utime: int64(0), Stime: int64(0), Rtime: int64(318437), BgUtime: int64(0), BgStime: int64(0),
				BgRtime: int64(0), SRun: int64(0), SInt: int64(2), SUnint: int64(0), SOth: int64(0), LogIdx: int64(939983), Rx: int64(0), Tx: int64(0)},
		},
		&parseComparison{
			line:   "e7a59f87-838c-48a0-bb82-bf97f5e8c79d 2016-07-05 14:19:10.239999912 82421087 [939984.528178]   201   201 D Kernel-Trace:      kworker/1:1-17    [001] ...2 939984.527405: phonelab_periodic_ctx_switch_info: cpu=1 pid=1563 tgid=1561 nice=-5 comm=su_daemon utime=1 stime=2 rtime=318437 bg_utime=3 bg_stime=4 bg_rtime=5 s_run=6 s_int=2 s_unint=7 s_oth=8 log_idx=939983 rx=9 tx=10",
			parser: parser,
			expected: &PhonelabPeriodicCtxSwitchInfo{
				Cpu: 1, Pid: 1563, Tgid: 1561, Nice: -5, Comm: "su_daemon", Utime: int64(1), Stime: int64(2), Rtime: int64(318437), BgUtime: int64(3), BgStime: int64(4),
				BgRtime: int64(5), SRun: int64(6), SInt: int64(2), SUnint: int64(7), SOth: int64(8), LogIdx: int64(939983), Rx: int64(9), Tx: int64(10)},
		},
		// Missing rx/tx
		&parseComparison{
			line:   "e7a59f87-838c-48a0-bb82-bf97f5e8c79d 2016-07-05 14:19:10.239999912 82421087 [939984.528178]   201   201 D Kernel-Trace:      kworker/1:1-17    [001] ...2 939984.527405: phonelab_periodic_ctx_switch_info: cpu=1 pid=1563 tgid=1561 nice=-5 comm=su_daemon utime=1 stime=2 rtime=318437 bg_utime=3 bg_stime=4 bg_rtime=5 s_run=6 s_int=2 s_unint=7 s_oth=8 log_idx=939983",
			parser: parser,
			expected: &PhonelabPeriodicCtxSwitchInfo{
				Cpu: 1, Pid: 1563, Tgid: 1561, Nice: -5, Comm: "su_daemon", Utime: int64(1), Stime: int64(2), Rtime: int64(318437), BgUtime: int64(3), BgStime: int64(4),
				BgRtime: int64(5), SRun: int64(6), SInt: int64(2), SUnint: int64(7), SOth: int64(8), LogIdx: int64(939983)},
		},
		// Invalid
		&parseComparison{
			line:          "e7a59f87-838c-48a0-bb82-bf97f5e8c79d 2016-07-05 14:19:10.239999912 82421087 [939984.528178]   201   201 D Kernel-Trace:      kworker/1:1-17    [001] ...2 939984.527405: phonelab_periodic_ctx_switch_info: cpu=1 pid=1563 tgid=1561 nice=-5 comm=su_daemon utime=1 stime=2 rtime=318437 bg_utime=3 bg_stime=4 bg_rtime=5 s_run=6 s_int=2 s_unint=7 s_oth",
			parser:        parser,
			subParseFails: true,
		},
	}
	commonTestParse(testConf, t)
}

func TestParsePhonelabPeriodicCtxSwitchMarker(t *testing.T) {
	t.Parallel()

	parser := NewKernelTraceParser()

	testConf := []*parseComparison{
		// Fail state
		&parseComparison{
			line:          "956dfa096f3dffaac02b2554fc508aa29d1fe21a        1468573870399   1468573870399.4 0aa2908d-ace5-4f2b-bbe5-1e2efa26e320    9264    79.527285       2016-07-15 09:11:10.399999      202     202     D       Kernel-Trace    kworker/1:1-3411  [001] ...2    79.526307: phonelab_periodic_ctx_switch_marker: BEGIAN cpu=1 count=0 log_idx=72",
			parser:        parser,
			subParseFails: true,
		},
		// Fail cpu
		&parseComparison{
			line:          "956dfa096f3dffaac02b2554fc508aa29d1fe21a        1468573870399   1468573870399.4 0aa2908d-ace5-4f2b-bbe5-1e2efa26e320    9264    79.527285       2016-07-15 09:11:10.399999      202     202     D       Kernel-Trace    kworker/1:1-3411  [001] ...2    79.526307: phonelab_periodic_ctx_switch_marker: BEGIN cpu=13030420520503023002 count=0 log_idx=72",
			parser:        parser,
			subParseFails: true,
		},
		// Negative cpu
		&parseComparison{
			line:          "956dfa096f3dffaac02b2554fc508aa29d1fe21a        1468573870399   1468573870399.4 0aa2908d-ace5-4f2b-bbe5-1e2efa26e320    9264    79.527285       2016-07-15 09:11:10.399999      202     202     D       Kernel-Trace    kworker/1:1-3411  [001] ...2    79.526307: phonelab_periodic_ctx_switch_marker: BEGIN cpu=-1 count=0 log_idx=72",
			parser:        parser,
			subParseFails: true,
		},
		// Fail count
		&parseComparison{
			line:          "956dfa096f3dffaac02b2554fc508aa29d1fe21a        1468573870399   1468573870399.4 0aa2908d-ace5-4f2b-bbe5-1e2efa26e320    9264    79.527285       2016-07-15 09:11:10.399999      202     202     D       Kernel-Trace    kworker/1:1-3411  [001] ...2    79.526307: phonelab_periodic_ctx_switch_marker: BEGIN cpu=1 count=1929319949124919491919 log_idx=72",
			parser:        parser,
			subParseFails: true,
		},

		// Negative count
		&parseComparison{
			line:          "956dfa096f3dffaac02b2554fc508aa29d1fe21a        1468573870399   1468573870399.4 0aa2908d-ace5-4f2b-bbe5-1e2efa26e320    9264    79.527285       2016-07-15 09:11:10.399999      202     202     D       Kernel-Trace    kworker/1:1-3411  [001] ...2    79.526307: phonelab_periodic_ctx_switch_marker: BEGIN cpu=1 count=-1 log_idx=72",
			parser:        parser,
			subParseFails: true,
		},
		// Fail log_idx
		&parseComparison{
			line:          "956dfa096f3dffaac02b2554fc508aa29d1fe21a        1468573870399   1468573870399.4 0aa2908d-ace5-4f2b-bbe5-1e2efa26e320    9264    79.527285       2016-07-15 09:11:10.399999      202     202     D       Kernel-Trace    kworker/1:1-3411  [001] ...2    79.526307: phonelab_periodic_ctx_switch_marker: BEGIN cpu=1 count=0 log_idx=95329539953923959235929359239293959599395935993939539593959199192191",
			parser:        parser,
			subParseFails: true,
		},
		// Negative log_idx
		&parseComparison{
			line:          "956dfa096f3dffaac02b2554fc508aa29d1fe21a        1468573870399   1468573870399.4 0aa2908d-ace5-4f2b-bbe5-1e2efa26e320    9264    79.527285       2016-07-15 09:11:10.399999      202     202     D       Kernel-Trace    kworker/1:1-3411  [001] ...2    79.526307: phonelab_periodic_ctx_switch_marker: BEGIN cpu=1 count=0 log_idx=-44",
			parser:        parser,
			subParseFails: true,
		},
		// Valid
		&parseComparison{
			line: "956dfa096f3dffaac02b2554fc508aa29d1fe21a        1468573870399   1468573870399.375       0aa2908d-ace5-4f2b-bbe5-1e2efa26e320    9635    79.533367       2016-07-15 09:11:10.399999      202     202     D	       Kernel-Trace    kworker/1:1-3411  [001] ...2    79.527064: phonelab_periodic_ctx_switch_marker: BEGIN cpu=1 count=369 log_idx=72",
			parser:   parser,
			expected: &PhonelabPeriodicCtxSwitchMarker{Cpu: 1, State: PPCSMBegin, Count: 369, LogIdx: int64(72)},
		},
		// Valid
		&parseComparison{
			line: "956dfa096f3dffaac02b2554fc508aa29d1fe21a        1468573870399   1468573870399.375       0aa2908d-ace5-4f2b-bbe5-1e2efa26e320    9635    79.533367       2016-07-15 09:11:10.399999      202     202     D	       Kernel-Trace    kworker/1:1-3411  [001] ...2    79.527064: phonelab_periodic_ctx_switch_marker: END cpu=1 count=369 log_idx=72",
			parser:   parser,
			expected: &PhonelabPeriodicCtxSwitchMarker{Cpu: 1, State: PPCSMEnd, Count: 369, LogIdx: int64(72)},
		},
	}

	commonTestParse(testConf, t)

}
