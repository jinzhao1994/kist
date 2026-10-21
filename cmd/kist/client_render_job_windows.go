package main

import (
	osexec "os/exec"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// containClientRenderer is called before the request is sent on stdin. The
// worker cannot start a converter until assignment to this non-inheritable job
// succeeds. Closing the parent's handle also covers an abrupt parent exit.
func containClientRenderer(command *osexec.Cmd) (func() error, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	_, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits)))
	if err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(command.Process.Pid))
	if err == nil {
		err = windows.AssignProcessToJobObject(job, process)
		windows.CloseHandle(process)
	}
	if err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	return func() error {
		defer windows.CloseHandle(job)
		// A worker can exit before its converter. Keep the caller's workspace and
		// original leases until the kernel reports that the entire job is empty.
		if err := windows.TerminateJobObject(job, 1); err != nil {
			return err
		}
		for {
			var accounting clientJobAccounting
			if err := windows.QueryInformationJobObject(job, windows.JobObjectBasicAccountingInformation, uintptr(unsafe.Pointer(&accounting)), uint32(unsafe.Sizeof(accounting)), nil); err != nil {
				return err
			}
			if accounting.ActiveProcesses == 0 {
				return nil
			}
			time.Sleep(time.Millisecond)
		}
	}, nil
}

// Layout of JOBOBJECT_BASIC_ACCOUNTING_INFORMATION, including trailing padding.
type clientJobAccounting struct {
	TotalUserTime, TotalKernelTime                                                 int64
	ThisPeriodTotalUserTime, ThisPeriodTotalKernelTime                             int64
	TotalPageFaultCount, TotalProcesses, ActiveProcesses, TotalTerminatedProcesses uint32
}
