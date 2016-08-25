#!/usr/bin/python

import os, re, sys, time, json
from datetime import datetime, date

year = str(date.today().year)

#test_result_dir = '/usr/local/google/home/zhoufang/test_logs'
test_result_file = 'build-log.txt'
kubelet_log_file = 'kubelet.log'
tracing_file = 'tracing.log'

regex_event = re.compile(r'[IW](?:\d{2}\d{2} \d{2}:\d{2}:\d{2}.\d{6}) .* Event\(.*\): type: \'(.*)\' reason: \'(.*)\' (.*)')
regex_event_msg = re.compile(r'pod: (.*), probe: (.*), timestamp: ([0-9]*)')
regex_test_start = re.compile(r'(?:.*): INFO: The test (.*) on (.*) starts at ([0-9]*).')
regex_test_end = re.compile(r'(?:.*): INFO: The test (.*) on (.*) ends at ([0-9]*).')

tracing_event_reason = "NodeTracing"
tracing_event_set = {"PodCreateFirstSeen", "PodCreateRunning"}

timeseries_result_tag = '[Result:TimeSeries]'
timeseries_finish_tag = '[Finish:TimeSeries]'
version = 'v1'


class TestTimeRange:
    def __init__(self, test, node, start_ts, end_ts):
        self.test = test
        self.node = node
        self.start_ts = start_ts
        self.end_ts = end_ts

    def in_range(self, ts):
        if ts >= self.start_ts and ts <= self.end_ts:
            return True
        return False


class TimeSeries:
    def __init__(self, node, test):
        self.labels = {
            'node': node,
            'test': test,
        }
        self.version = version
        self.op_series = {}

    def append_op_series(self, key, value):
        if key not in self.op_series:
            self.op_series[key] = []
        self.op_series[key].append(value)


    def load_tracing(self, test_result_dir, build, test_time_range):
        with open(os.path.join(test_result_dir, str(build), test_time_range.node, kubelet_log_file)) as f:
            for line in f.readlines():
                match_obj = regex_event.match(line)
                # find an tracing event in log
                if match_obj != None and match_obj.group(2) == tracing_event_reason:
                    event_type = match_obj.group(1)
                    event_reason = match_obj.group(2)
                    event_msg = match_obj.group(3)

                    msg_match_obj = regex_event_msg.match(event_msg)
                    if msg_match_obj == None:
                        print 'Error: parsing event message error :{}'.format(event_msg)
                        sys.exit(1)

                    pod = msg_match_obj.group(1)
                    probe = msg_match_obj.group(2)
                    ts_unixnano = int(msg_match_obj.group(3))

                    if test_time_range.in_range(ts_unixnano):
                        self.append_op_series(probe, ts_unixnano)

        time_series_str = json.dumps(self.__dict__)
        return timeseries_result_tag + time_series_str + '\n\n' + timeseries_finish_tag + '\n'


def load_test_ts_range(test_result_dir, build):
    test_time_range_list = []
    with open(os.path.join(test_result_dir, str(build), test_result_file)) as f:
        time_range = None
        for line in f.readlines():
            match_obj = regex_test_start.match(line)
            if match_obj != None:
                test = match_obj.group(1)
                node = match_obj.group(2)
                ts_unixnano = int(match_obj.group(3))
                if time_range == None:
                    time_range = TestTimeRange(test, node, ts_unixnano, 0)
                else:
                    print 'Error: find unhandled test start timestamp'
                    sys.exit(1)
                continue

            match_obj = regex_test_end.match(line)
            if match_obj != None:
                test = match_obj.group(1)
                node = match_obj.group(2)
                ts_unixnano = int(match_obj.group(3))
                if time_range!=None and time_range.test==test and time_range.node==node and time_range.end_ts==0:
                    time_range.end_ts = ts_unixnano
                    test_time_range_list.append(time_range)
                    time_range = None
                else:
                    print 'Error: start and end timestamps mismatch'
                    sys.exit(1)
    return test_time_range_list


def main():
    test_result_dir = sys.argv[1]
    build = sys.argv[2]

    print 'parse_kubelet_log.py: parsing tracing data from kubelet.log'
    
    tracing_file_path = os.path.join(test_result_dir, str(build), tracing_file)
    # Parse tracing data only if it does not exit.
    if not os.path.isfile(tracing_file_path):
        with open(tracing_file_path, 'a') as f:
            f.write('\nTracing time series data from kubelet.log:\n\n')
            for test_ts_range in load_test_ts_range(test_result_dir, build):
                time_series = TimeSeries(test_ts_range.node, test_ts_range.test)
                f.write(time_series.load_tracing(test_result_dir, build, test_ts_range))

if __name__ == '__main__':
    main()
