package router

import (
	"context"
	"fmt"
	"net"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	admissionapi "k8s.io/pod-security-admission/api"

	routeclientset "github.com/openshift/client-go/route/clientset/versioned"

	exutil "github.com/openshift/origin/test/extended/util"
	exurl "github.com/openshift/origin/test/extended/util/url"
)

var _ = g.Describe("[sig-network][Feature:Router][apigroup:route.openshift.io]", func() {
	defer g.GinkgoRecover()
	var (
		oc          *exutil.CLI
		ns          string
		routerImage string
		isFIPS      bool
	)
	const (
		pemData = `-----BEGIN CERTIFICATE-----
MIIDIjCCAgqgAwIBAgIBBjANBgkqhkiG9w0BAQUFADCBoTELMAkGA1UEBhMCVVMx
CzAJBgNVBAgMAlNDMRUwEwYDVQQHDAxEZWZhdWx0IENpdHkxHDAaBgNVBAoME0Rl
ZmF1bHQgQ29tcGFueSBMdGQxEDAOBgNVBAsMB1Rlc3QgQ0ExGjAYBgNVBAMMEXd3
dy5leGFtcGxlY2EuY29tMSIwIAYJKoZIhvcNAQkBFhNleGFtcGxlQGV4YW1wbGUu
Y29tMB4XDTE2MDExMzE5NDA1N1oXDTI2MDExMDE5NDA1N1owfDEYMBYGA1UEAxMP
d3d3LmV4YW1wbGUuY29tMQswCQYDVQQIEwJTQzELMAkGA1UEBhMCVVMxIjAgBgkq
hkiG9w0BCQEWE2V4YW1wbGVAZXhhbXBsZS5jb20xEDAOBgNVBAoTB0V4YW1wbGUx
EDAOBgNVBAsTB0V4YW1wbGUwgZ8wDQYJKoZIhvcNAQEBBQADgY0AMIGJAoGBAM0B
u++oHV1wcphWRbMLUft8fD7nPG95xs7UeLPphFZuShIhhdAQMpvcsFeg+Bg9PWCu
v3jZljmk06MLvuWLfwjYfo9q/V+qOZVfTVHHbaIO5RTXJMC2Nn+ACF0kHBmNcbth
OOgF8L854a/P8tjm1iPR++vHnkex0NH7lyosVc/vAgMBAAGjDTALMAkGA1UdEwQC
MAAwDQYJKoZIhvcNAQEFBQADggEBADjFm5AlNH3DNT1Uzx3m66fFjqqrHEs25geT
yA3rvBuynflEHQO95M/8wCxYVyuAx4Z1i4YDC7tx0vmOn/2GXZHY9MAj1I8KCnwt
Jik7E2r1/yY0MrkawljOAxisXs821kJ+Z/51Ud2t5uhGxS6hJypbGspMS7OtBbw7
8oThK7cWtCXOldNF6ruqY1agWnhRdAq5qSMnuBXuicOP0Kbtx51a1ugE3SnvQenJ
nZxdtYUXvEsHZC/6bAtTfNh+/SwgxQJuL2ZM+VG3X2JIKY8xTDui+il7uTh422lq
wED8uwKl+bOj6xFDyw4gWoBxRobsbFaME8pkykP1+GnKDberyAM=
-----END CERTIFICATE-----
-----BEGIN RSA PRIVATE KEY-----
MIICWwIBAAKBgQDNAbvvqB1dcHKYVkWzC1H7fHw+5zxvecbO1Hiz6YRWbkoSIYXQ
EDKb3LBXoPgYPT1grr942ZY5pNOjC77li38I2H6Pav1fqjmVX01Rx22iDuUU1yTA
tjZ/gAhdJBwZjXG7YTjoBfC/OeGvz/LY5tYj0fvrx55HsdDR+5cqLFXP7wIDAQAB
AoGAfE7P4Zsj6zOzGPI/Izj7Bi5OvGnEeKfzyBiH9Dflue74VRQkqqwXs/DWsNv3
c+M2Y3iyu5ncgKmUduo5X8D9To2ymPRLGuCdfZTxnBMpIDKSJ0FTwVPkr6cYyyBk
5VCbc470pQPxTAAtl2eaO1sIrzR4PcgwqrSOjwBQQocsGAECQQD8QOra/mZmxPbt
bRh8U5lhgZmirImk5RY3QMPI/1/f4k+fyjkU5FRq/yqSyin75aSAXg8IupAFRgyZ
W7BT6zwBAkEA0A0ugAGorpCbuTa25SsIOMxkEzCiKYvh0O+GfGkzWG4lkSeJqGME
keuJGlXrZNKNoCYLluAKLPmnd72X2yTL7wJARM0kAXUP0wn324w8+HQIyqqBj/gF
Vt9Q7uMQQ3s72CGu3ANZDFS2nbRZFU5koxrggk6lRRk1fOq9NvrmHg10AQJABOea
pgfj+yGLmkUw8JwgGH6xCUbHO+WBUFSlPf+Y50fJeO+OrjqPXAVKeSV3ZCwWjKT4
9viXJNJJ4WfF0bO/XwJAOMB1wQnEOSZ4v+laMwNtMq6hre5K8woqteXICoGcIWe8
u3YLAbyW/lHhOCiZu2iAI8AbmXem9lW6Tr7p/97s0w==
-----END RSA PRIVATE KEY-----
`
	)

	g.AfterEach(func() {
		if g.CurrentGinkgoTestDescription().Failed {
			client := routeclientset.NewForConfigOrDie(oc.AdminConfig()).RouteV1().Routes(ns)
			if routes, _ := client.List(context.Background(), metav1.ListOptions{}); routes != nil {
				outputIngress(routes.Items...)
			}
			selector, err := labels.Parse("test=router-scoped")
			if err != nil {
				panic(err)
			}
			exutil.DumpPodsCommand(oc.AdminKubeClient(), ns, selector, "cat /etc/crypto-policies/back-ends/opensslcnf.config")
			exutil.DumpPodLogsStartingWith("router-", oc)
		}
	})

	oc = exutil.NewCLIWithPodSecurityLevel("router-certs", admissionapi.LevelBaseline)

	g.BeforeEach(func() {
		ns = oc.Namespace()

		var err error
		routerImage, err = exutil.FindRouterImage(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		isFIPS, err = exutil.IsFIPS(oc.AdminKubeClient().CoreV1())
		o.Expect(err).NotTo(o.HaveOccurred())

		configPath := exutil.FixturePath("testdata", "router", "router-common.yaml")
		err = oc.AsAdmin().Run("new-app").Args("-f", configPath).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.When("FIPS is enabled", func() {
		g.Describe("the HAProxy router", func() {
			g.It("should not work when configured with a 1024-bit RSA key [apigroup:template.openshift.io]", func() {
				if !isFIPS {
					g.Skip("skipping on non-FIPS cluster")
				}

				configPath := exutil.FixturePath("testdata", "router", "router-scoped.yaml")
				g.By(fmt.Sprintf("creating a router from a config file %q", configPath))
				err := oc.AsAdmin().Run("new-app").Args("-f", configPath,
					"-p=IMAGE="+routerImage,
					`-p=ROUTER_NAME=test-1024bit`,
					`-p=DEFAULT_CERTIFICATE=`+pemData,
				).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				ns := oc.KubeFramework().Namespace.Name
				execPod := exutil.CreateExecPodOrFail(oc.AdminKubeClient(), ns, "execpod")
				defer func() {
					oc.AdminKubeClient().CoreV1().Pods(ns).Delete(context.Background(), execPod.Name, *metav1.NewDeleteOptions(1))
				}()

				var routerIP string
				err = wait.Poll(time.Second, changeTimeoutSeconds*time.Second, func() (bool, error) {
					pod, err := oc.KubeFramework().ClientSet.CoreV1().Pods(oc.KubeFramework().Namespace.Name).Get(context.Background(), "router-scoped", metav1.GetOptions{})
					if err != nil {
						return false, err
					}
					routerIP = pod.Status.PodIP
					podIsReady := podConditionStatus(pod, corev1.PodReady)

					return len(routerIP) != 0 && podIsReady == corev1.ConditionTrue, nil
				})
				o.Expect(err).To(o.HaveOccurred())
			})
		})
	})

	g.When("FIPS is disabled", func() {
		g.Describe("the HAProxy router", func() {
			g.It("should serve routes when configured with a 1024-bit RSA key [apigroup:template.openshift.io]", func() {
				if isFIPS {
					g.Skip("skipping on FIPS cluster")
				}

				configPath := exutil.FixturePath("testdata", "router", "router-scoped.yaml")
				g.By(fmt.Sprintf("creating a router from a config file %q", configPath))
				err := oc.AsAdmin().Run("new-app").Args("-f", configPath,
					"-p=IMAGE="+routerImage,
					`-p=ROUTER_NAME=test-1024bit`,
					`-p=DEFAULT_CERTIFICATE=`+pemData,
				).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				ns := oc.KubeFramework().Namespace.Name
				execPod := exutil.CreateExecPodOrFail(oc.AdminKubeClient(), ns, "execpod")
				defer func() {
					oc.AdminKubeClient().CoreV1().Pods(ns).Delete(context.Background(), execPod.Name, *metav1.NewDeleteOptions(1))
				}()

				var routerIP string
				err = wait.Poll(time.Second, changeTimeoutSeconds*time.Second, func() (bool, error) {
					pod, err := oc.KubeFramework().ClientSet.CoreV1().Pods(oc.KubeFramework().Namespace.Name).Get(context.Background(), "router-scoped", metav1.GetOptions{})
					if err != nil {
						return false, err
					}
					routerIP = pod.Status.PodIP
					podIsReady := podConditionStatus(pod, corev1.PodReady)

					return len(routerIP) != 0 && podIsReady == corev1.ConditionTrue, nil
				})
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("waiting for the router's healthz endpoint to respond")
				healthzURI := fmt.Sprintf("http://%s/healthz", net.JoinHostPort(routerIP, "1936"))
				healthzt := exurl.NewTester(oc.AdminKubeClient(), ns).WithErrorPassthrough(true)
				defer healthzt.Close()
				healthzt.Within(
					time.Minute,
					exurl.Expect("GET", healthzURI).SkipTLSVerification().HasStatusCode(200),
				)

				g.By("waiting for the route to respond")
				url := "https://first.example.com/Letter"
				t := exurl.NewTester(oc.AdminKubeClient(), ns).WithErrorPassthrough(true)
				defer t.Close()
				t.Within(
					time.Minute,
					exurl.Expect("GET", url).Through(routerIP).SkipTLSVerification().HasStatusCode(200),
				)
			})
		})
	})
})
