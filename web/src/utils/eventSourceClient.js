/**
 * XrayEventSource.js
 * 用于处理Xray版本管理相关的SSE事件
 */

class XrayEventSource {
  constructor() {
    this.eventSource = null;
    this.retryCount = 0;
    this.maxRetries = 5;
    this.retryInterval = 3000; // 3秒
    this.connected = false;
    this.reconnecting = false;
  }

  /**
   * 初始化SSE连接
   */
  init() {
    if (this.eventSource) {
      // 如果已经连接，先关闭之前的连接
      this.close();
    }

    try {
      this.eventSource = new EventSource('/api/sse/xray-events');

      // 添加事件监听器
      this.eventSource.addEventListener('connected', this.handleConnected.bind(this));
      this.eventSource.addEventListener('xray-progress', this.handleXrayProgress.bind(this));
      
      // 错误处理
      this.eventSource.onerror = this.handleError.bind(this);

      console.log('XrayEventSource: Connecting to SSE endpoint');
    } catch (error) {
      console.error('XrayEventSource: Failed to initialize SSE connection', error);
    }
  }

  /**
   * 关闭SSE连接
   */
  close() {
    if (this.eventSource) {
      this.eventSource.close();
      this.eventSource = null;
      this.connected = false;
      console.log('XrayEventSource: Connection closed');
    }
  }

  /**
   * 处理连接成功事件
   */
  handleConnected() {
    this.connected = true;
    this.retryCount = 0;
    console.log('XrayEventSource: Connected to server');
  }

  /**
   * 处理Xray进度事件
   * @param {Event} event 
   */
  handleXrayProgress(event) {
    try {
      const data = JSON.parse(event.data);
      
      // 创建自定义事件
      const customEvent = new CustomEvent('xray-download-progress', {
        detail: data
      });
      
      // 分发事件
      window.dispatchEvent(customEvent);
      
      console.log('XrayEventSource: Received progress event', data);
    } catch (error) {
      console.error('XrayEventSource: Failed to parse progress event', error);
    }
  }

  /**
   * 处理连接错误
   */
  handleError(error) {
    console.error('XrayEventSource: Connection error', error);
    
    if (this.eventSource) {
      this.eventSource.close();
      this.eventSource = null;
      this.connected = false;
    }
    
    // 尝试重连
    if (!this.reconnecting && this.retryCount < this.maxRetries) {
      this.reconnecting = true;
      this.retryCount++;
      
      console.log(`XrayEventSource: Reconnecting (${this.retryCount}/${this.maxRetries}) in ${this.retryInterval / 1000}s`);
      
      setTimeout(() => {
        this.reconnecting = false;
        this.init();
      }, this.retryInterval);
    } else if (this.retryCount >= this.maxRetries) {
      console.error('XrayEventSource: Max retries reached, giving up');
    }
  }
}

// 创建单例
const xrayEventSource = new XrayEventSource();

// 导出单例
export default xrayEventSource; 
