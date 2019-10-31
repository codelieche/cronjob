package tcpmonitor

import (
	"fmt"
	"log"
	"net"
	"time"
)

// ExecuteMonitorLoop 执行tcp监控循环函数
func (tcpMonitor *TCPMonitor) ExecuteMonitorLoop() {
	// 执行监控进入这个循环即可
	// 定义变量
	var (
		monitorExecuteInfoMap map[string]*monitorExecuteInfo // 监控执行信息map
		address               string                         // 监控的地址
		executeInfo           *monitorExecuteInfo            // 监控执行信息
		isExist               bool                           // 是否存在
		err                   error                          // 错误信息
	)
	// 第1步：对传入的值做校验：检查值、设置默认值等操作
	if err = tcpMonitor.validate(); err != nil {
		log.Println(err.Error())
		return
	}

	// 第2步：准备
	monitorExecuteInfoMap = make(map[string]*monitorExecuteInfo)
	// log.Println(tcpMonitor.lock, tcpMonitor.wg)
	tcpMonitor.Status = "Runing"

	// 第3步：执行监控
	// tcpMonitor.wg = sync.WaitGroup
	for {
		// 第4步: 对每个主机+端口进行检查
		for _, host := range tcpMonitor.Hosts {
			// 4-1：准备监控需要的数据
			// address: 要监控的地址：host:port
			address = fmt.Sprintf("%s:%d", host, tcpMonitor.Port)
			// executeInfo: 监控执行信息
			if executeInfo, isExist = monitorExecuteInfoMap[address]; !isExist {
				// 不存在于map中，那么就是第一次执行，我们实例化一个executeInfo
				executeInfo = &monitorExecuteInfo{
					address:                address,
					count:                  0, // 这个以及后面的值其实都用默认值即可
					successTimes:           0,
					errorTimes:             0,
					needSendErrorMessage:   false,
					errorMessageSended:     false,
					needSendRecoverMessage: false,
					recoverMessageSended:   false,
				}
				// 加入到map中
				monitorExecuteInfoMap[address] = executeInfo
			}
			// 第5步：执行监控：对当前address进行尝试连接
			tcpMonitor.wg.Add(1)
			tcpMonitor.Count++
			go tcpMonitor.executeMonitor(address, executeInfo)
		}
		// 等待启动的监控协程，全部执行完毕
		tcpMonitor.wg.Wait()

		// 第6步：判断是否都执行成功了
		if tcpMonitor.monitorSuccessCount >= len(tcpMonitor.Hosts) {
			// 执行监控成功的数量 >= 需要监控的主机数：成功
			log.Println("所有主机都检查正常，程序退出!")
			// 只有道这里程序才算正常
			return
		}

		// 第7步：延时一会后，程序继续执行下一轮检查
		time.Sleep(time.Duration(tcpMonitor.Interval) * time.Second)
	}
	// log.Println("程序跳出了for循环，执行到这里就表示出问题了！")
}

// 执行监控函数
// executeMonitor 执行tcp监控的函数
func (tcpMonitor *TCPMonitor) executeMonitor(address string, info *monitorExecuteInfo) {
	// 定义变量：
	var (
		duration time.Duration // 超时时间(秒)
		conn     net.Conn      // tcp连接
		err      error         // 错误
	)
	log.Println(address)
	// 第1步：重点，记得执行wg.Done
	defer tcpMonitor.wg.Done()

	// 第2步：执行检查
	duration = time.Duration(tcpMonitor.Timemout) * time.Second
	if conn, err = net.DialTimeout("tcp", address, duration); err != nil {
		// 情况1： 执行监控就报错了
		log.Printf("%s: 连接出现了错误: %s\n", address, err)
		// 对错误数赋值，且重置当前执行的成功数
		info.errorTimes++
		info.successTimes = 0

		// 只要出错：就重新统计监控成功数
		tcpMonitor.lock.Lock()
		tcpMonitor.monitorSuccessCount = 0
		tcpMonitor.lock.Unlock() // 一定记得释放锁

		// 判断是否需要重置times
		if info.needSendRecoverMessage && info.recoverMessageSended {
			// 这种情况是：曾经出过错误，但是恢复了；然后又出错了。需要重置相关信息
			info.errorTimes = 1
			info.successTimes = 0
			info.needSendErrorMessage = false
			info.errorMessageSended = false
			info.needSendRecoverMessage = false
			info.recoverMessageSended = false
		}

		// 判断是否需要【设置】发送错误信息
		if info.errorTimes >= tcpMonitor.Times {
			info.needSendErrorMessage = true
		} else {
			info.needSendErrorMessage = false
		}

		// 判断是否需要【发送】错误信息
		if info.needSendErrorMessage && !info.errorMessageSended {
			// 需要发送，且已发送为false：就需要发送错误信息
			msg := fmt.Sprintf("地址: %s\n错误: %s", address, err.Error())
			if success, err := tcpMonitor.SendErrorMessage(msg); err != nil {
				log.Printf("%s: 发送消息出错: %s\n", address, err.Error())
			} else {
				if success {
					// 如果发送成功了，那么设置errMessageSended为true
					info.errorMessageSended = true
					log.Printf("发送%s出错消息，成功！\n", address)
				}
			}
		}
		// 延时后继续执行下一次

	} else {
		// 情况2：连接执行成功
		log.Printf("%s -> %s: 连接成功\n", conn.LocalAddr(), conn.RemoteAddr())
		// 判断是否以前出过错
		if info.errorTimes > 0 {
			// 曾经出过错
			info.successTimes++
			// 如果发送过错误消息了，判断是否需要发送恢复消息
			if info.successTimes >= tcpMonitor.Times {
				if info.needSendErrorMessage && info.errorMessageSended {
					// 需要发送出错恢复消息
					info.needSendRecoverMessage = true
				} else {
					info.needSendRecoverMessage = false
					// 设置成功个数+1
					tcpMonitor.lock.Lock()
					tcpMonitor.monitorSuccessCount++
					tcpMonitor.lock.Unlock() // 注意释放锁
				}

				// 如果需要发送错误恢复消息
				if info.needSendRecoverMessage && !info.recoverMessageSended {
					// 执行发送恢复消息
					log.Printf("%s: 需要执行发送恢复消息！\n", address)
					msg := fmt.Sprintf("%s已经恢复", address)
					if success, err := tcpMonitor.SendRecoverMessage(msg); err != nil {
						log.Println("发送恢复消息出错：", msg)
						// 既然恢复了，可以不重复发送恢复消息
					} else {
						if success {
							// 设置相关信息
							info.recoverMessageSended = true
							// 执行发送消息成功才返回程序
							log.Printf("%s: 发送恢复消息成功", address)

							tcpMonitor.lock.Lock()
							tcpMonitor.monitorSuccessCount++
							tcpMonitor.lock.Unlock() // 注意释放锁
						} else {
							log.Println("发送恢复消息没有成功：", address)
						}
					}
				}
			} else {
				// 还需要执行一次监控
				log.Println("虽然监控ok了，但是还需要检查一次，sleep后继续执行检查：", address)

				// 也等于有错，需要重置成功数
				tcpMonitor.lock.Lock()
				tcpMonitor.monitorSuccessCount = 0
				tcpMonitor.lock.Unlock() // 注意释放锁
			}

		} else {
			// 情况3：未出现过错误：服务大部分正常情况
			tcpMonitor.lock.Lock()
			tcpMonitor.monitorSuccessCount++
			tcpMonitor.lock.Unlock() // 注意释放锁
		}
	}

	// 打印出一点信息：
	if info.errorTimes > 0 && info.successTimes <= tcpMonitor.Times {
		log.Printf("%s: 检查次数: %d, 出错次数: %d，成功次数: %d\n", info.address,
			info.count, info.errorTimes, info.successTimes)
	}

}
