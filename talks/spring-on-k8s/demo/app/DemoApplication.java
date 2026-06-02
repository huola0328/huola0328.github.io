// 演示用最小 Spring Boot 应用：一个会读取 ConfigMap 注入值的接口 + 健康端点。
// 放到标准 Spring Boot 工程的 src/main/java/com/example/demo/ 下即可。
package com.example.demo;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

@SpringBootApplication
@RestController
public class DemoApplication {

    // app.greeting 来自 ConfigMap 注入的环境变量 APP_GREETING
    @Value("${app.greeting:Hello (default)}")
    private String greeting;

    @GetMapping("/")
    public String index() {
        return greeting + " | host=" + System.getenv().getOrDefault("HOSTNAME", "unknown");
    }

    // 一个故意吃 CPU 的接口，用于演示 HPA 自动扩容
    @GetMapping("/burn")
    public String burn() {
        long end = System.currentTimeMillis() + 2000;
        double x = 0;
        while (System.currentTimeMillis() < end) {
            x += Math.sqrt(Math.random());
        }
        return "burned cpu: " + x;
    }

    public static void main(String[] args) {
        SpringApplication.run(DemoApplication.class, args);
    }
}
