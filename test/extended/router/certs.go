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
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	admissionapi "k8s.io/pod-security-admission/api"
	utilpointer "k8s.io/utils/pointer"

	routeclientset "github.com/openshift/client-go/route/clientset/versioned"

	exutil "github.com/openshift/origin/test/extended/util"
	exurl "github.com/openshift/origin/test/extended/util/url"
)

const (
	defaultPemData = `
-----BEGIN CERTIFICATE-----
MIIDuTCCAqGgAwIBAgIUZYD30F0sJl7HqxE7gAequtxk/HowDQYJKoZIhvcNAQEL
BQAwgaExCzAJBgNVBAYTAlVTMQswCQYDVQQIDAJTQzEVMBMGA1UEBwwMRGVmYXVs
dCBDaXR5MRwwGgYDVQQKDBNEZWZhdWx0IENvbXBhbnkgTHRkMRAwDgYDVQQLDAdU
ZXN0IENBMRowGAYDVQQDDBF3d3cuZXhhbXBsZWNhLmNvbTEiMCAGCSqGSIb3DQEJ
ARYTZXhhbXBsZUBleGFtcGxlLmNvbTAeFw0yMjAxMjgwMjU0MDlaFw0zMjAxMjYw
MjU0MDlaMHwxGDAWBgNVBAMMD3d3dy5leGFtcGxlLmNvbTELMAkGA1UECAwCU0Mx
CzAJBgNVBAYTAlVTMSIwIAYJKoZIhvcNAQkBFhNleGFtcGxlQGV4YW1wbGUuY29t
MRAwDgYDVQQKDAdFeGFtcGxlMRAwDgYDVQQLDAdFeGFtcGxlMIIBIjANBgkqhkiG
9w0BAQEFAAOCAQ8AMIIBCgKCAQEA71W7gdEnM+Nm4/SA/4jEJ2SPQfVjkCMsIYGO
WrLLHq23HkMGstQoPyBnjLY8LmkKQsNhhWGRMWQz6+yGKgI1gh8huhfocuw+HODE
K3ugP/3DlaVEQlIQbVzwxDx+K78UqZHecQAJfvakuS/JThxsMf8/pqLuhjAf+t9N
k0CO8Z6mNVALtSvyQ+e+zjmzepVtu6WmtJ+8zW9dBQEmg0QCfWFd06836LrfixLk
vTRgCn0lzTuj7rSuGjY45JDIvKK4jZGQJKsYN59Wxg1d2CEoXBUJOJjecVdS3NhY
ubHNdcm+6Equ5ZmyVEkBmv462rOcednsHU6Ggt/vWSe05EOPVQIDAQABow0wCzAJ
BgNVHRMEAjAAMA0GCSqGSIb3DQEBCwUAA4IBAQCHI+fkEr27bJ2IMtFuHpSLpFF3
E4R5oVHt8XjflwKmuclyyLa8Z7nXnuvQLHa4jwf0tWUixsmtOyQN4tBI/msMk2PF
+ao2amcPoIo2lAg63+jFsIzkr2MEXBPu09wwt86e3XCoqmqT1Psnihh+Ys9KIPnc
wMr9muGkOh03O61vo71iaV17UKeGM4bzod333pSQIXLdYnoOuvmKdCsnD00lADoI
93DmG/4oYR/mD93QjxPFPDxDxR4isvWGoj7iXx7CFkN7PR9B3IhZt+T//ddeau3y
kXK0iSxOhyaqHvl15hHQ8tKPBBJRSDVU4qmaqAYWRXr65yxBoelHhTJQ6Gt4
-----END CERTIFICATE-----
-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQDvVbuB0Scz42bj
9ID/iMQnZI9B9WOQIywhgY5assserbceQway1Cg/IGeMtjwuaQpCw2GFYZExZDPr
7IYqAjWCHyG6F+hy7D4c4MQre6A//cOVpURCUhBtXPDEPH4rvxSpkd5xAAl+9qS5
L8lOHGwx/z+mou6GMB/6302TQI7xnqY1UAu1K/JD577OObN6lW27paa0n7zNb10F
ASaDRAJ9YV3Trzfout+LEuS9NGAKfSXNO6PutK4aNjjkkMi8oriNkZAkqxg3n1bG
DV3YIShcFQk4mN5xV1Lc2Fi5sc11yb7oSq7lmbJUSQGa/jras5x52ewdToaC3+9Z
J7TkQ49VAgMBAAECggEAaCBzqOI3XSLlo+2/pe158e2VSkwZ2h8DVzyHk7xQFPPd
RKRCqNEXBYfypUyv2D1JAo0Aw8gUJFoFIPLR2DsHzqn+wXkfX8iaqXO8xXJO4Shl
zJiPnw8XKI2UDryG5D+JHNFi5uTuPLfQKOW6fmptRD9aEQS4I9eSQlKe7J7c0g+t
pCR1vCp6ZMFIXDgpHhquArI1fjA36nWK0dJkaO9LrTYPgeMIr0KFjEF+W3UPh/af
uw/KLjzyzHExwfVBcGZonb6rG1nU/7isUHqK75OhOKDcXpv+7NCBYZ6fu4COlE0O
+yGztbRXojWo1upKzzGPM+yoLyNA1aSljpCGOCSljQKBgQD+4i5FzRQ+e1XZxvUt
izypHHQcc7y9DfwKTwLXb9EUhmGCmrxVIuM+gm5N/Y/eXDjqtR2bqg7iIFjj3KTS
f9djCYT8FqlTtyDBk/qFNLchDX/mrykOuhqIXfT7JpQbk5+qkCy8k2ZJMl2ToNXA
WRqRCP4oa1WJMmoJFwo3BIVRIwKBgQDwYh2ryrs/QFE0W082oHAQ3Nrce5JmOtFp
70X/v8zZ8ESdeo7KOS0tNLeirBxlDGvUAesKwUHU1YwTgWhl/DkoPtv9INgT8kxS
VRcrix9kq62uiD+TKI732mwoG36keJdRECrQYRYjX+mf364EI+DeNmbPs3xsigaF
Zdbg+umxJwKBgF4fFelOvuAH2X8PGnDUDvV//VyYXKUPqfgAj1MRBotmyFFbZJqn
xHTL44HHVb5OHfKGKUXXeaGFQm36h573+Iio9kPE9ohkgqMZSxSvj8ST4JxGKIo4
rR2YXKP17hF05SwuC2cjo0z6XVXruaNLBCV0xa4VXMPKKx/qMyp37+czAoGBAL8c
woo6e/QlpmoBzlCX7YD6leaFODeeu6+FVBmo26zJoUOylKOiIZC3QOhL/ac44OGF
ROEgFL6pqNw5Hk824BpnH294FVKGaLdsfydXTHY1J7iDCkhtDn1vYl3gvib02RjR
ybgx9+/X6V3579fKzpTcm5C2Gk4Qzm5wMQ5dbj4xAoGBANYzYbBu8bItAEE6ohgf
D27SPW7VJsHGzbgRNC2SGCBzo3XaTJ0A8IMP+ghl5ndCJdLBz2FpeZLQvxOuopQD
J5dJXQxp7y20vh2C1e3wTPlA5CHHKpU1JZAe4THCJUg+EPwa4I+BOlvp71EB7BaH
bk65iLoLrUSkxMDi46qTAs5K
-----END PRIVATE KEY-----
`
	// Commands to recreate this cert (OpenSSL CLI v3.0.9):
	//    openssl req -x509 -newkey rsa:1024 -days 3650 -keyout exampleca.key -out exampleca.crt -nodes -subj '/C=US/ST=SC/L=Default City/O=Default Company Ltd/OU=Test CA/CN=www.exampleca.com/emailAddress=example@example.com'
	//    openssl req -newkey rsa:1024 -nodes -keyout example.key -out example.csr -subj '/CN=www.example.com/ST=SC/C=US/emailAddress=example@example.com/O=Example/OU=Example'
	//    openssl x509 -req -days 3650 -in example.csr -CA exampleca.crt -CAcreateserial -CAkey exampleca.key -extensions ext -extfile <(echo $'[ext]\nbasicConstraints = CA:FALSE\nsubjectKeyIdentifier = none\nauthorityKeyIdentifier = none') -out example.crt
	//    cat example.crt example.key
	pemData1024 = `-----BEGIN CERTIFICATE-----
MIICtDCCAh2gAwIBAgIUH16Mfgnwdr61PmIz5qrkoj70mK4wDQYJKoZIhvcNAQEL
BQAwgaExCzAJBgNVBAYTAlVTMQswCQYDVQQIDAJTQzEVMBMGA1UEBwwMRGVmYXVs
dCBDaXR5MRwwGgYDVQQKDBNEZWZhdWx0IENvbXBhbnkgTHRkMRAwDgYDVQQLDAdU
ZXN0IENBMRowGAYDVQQDDBF3d3cuZXhhbXBsZWNhLmNvbTEiMCAGCSqGSIb3DQEJ
ARYTZXhhbXBsZUBleGFtcGxlLmNvbTAeFw0yMzEyMTkxNTE1MjhaFw0zMzEyMTYx
NTE1MjhaMHwxGDAWBgNVBAMMD3d3dy5leGFtcGxlLmNvbTELMAkGA1UECAwCU0Mx
CzAJBgNVBAYTAlVTMSIwIAYJKoZIhvcNAQkBFhNleGFtcGxlQGV4YW1wbGUuY29t
MRAwDgYDVQQKDAdFeGFtcGxlMRAwDgYDVQQLDAdFeGFtcGxlMIGfMA0GCSqGSIb3
DQEBAQUAA4GNADCBiQKBgQC1ruE/Tl2exIRNwWBo0+HaN44d7l5Xrul3C63RFRFN
ZDMpnFtYiG0ruz/C9zpIqob0OQbwckB1ifjgiTq82sK7bMdHdQKn+Xe+jFoVTvRO
bW8SwbqNfL9/JLnjCUsWTGkn25WMHLztZ8GSn8DlE1arQ7UuuuPgW1OkkPpjKHxM
VQIDAQABow0wCzAJBgNVHRMEAjAAMA0GCSqGSIb3DQEBCwUAA4GBAAC1djh1f61x
Liam71j4d1K23ejwxFLD43Wa7+NEKxLg7+lsKLRC84lXq/6cf78Jqf8+f3duHyl+
uLaA4UGYvBUuB7edJCqv0/e0dmGrkwbtGOMxBKcp0I027REuwHlW7kx2LBzrISB6
pVsYNM8wKkTi9wchCqRW6AElpE4fd2Wk
-----END CERTIFICATE-----
-----BEGIN PRIVATE KEY-----
MIICeAIBADANBgkqhkiG9w0BAQEFAASCAmIwggJeAgEAAoGBALWu4T9OXZ7EhE3B
YGjT4do3jh3uXleu6XcLrdEVEU1kMymcW1iIbSu7P8L3OkiqhvQ5BvByQHWJ+OCJ
Orzawrtsx0d1Aqf5d76MWhVO9E5tbxLBuo18v38kueMJSxZMaSfblYwcvO1nwZKf
wOUTVqtDtS664+BbU6SQ+mMofExVAgMBAAECgYBmxZYFCX9L4D42/bxbj/+iQOrT
Y5NaZkcKYEDilNhEvvlyAFBrtECNDE71KoR9tnjAjcGvIfH0iyeNXBMt4VFlYAsv
sGzZRnbKq9uquicEUnKwmBfAfD8/ar+KK/8qDQgzb3Eknu1bDiC08QisXaqjLRF9
P7eOj1F78hiYLu/kAQJBANuc7E5m4ymVq5ej2aLf1vxeOs3GKeVt9wHAODjxOabw
JDn3n9bqFALOmYmfA5Y8dPr37mfyepb+tCNyEt7lcNECQQDTySD0P/88toi3rcph
0faoe3F01g5goQCcE7VXTXINUMsei/7x1t5Jd4l7WwVaeKxXn3V10uoxtt0to6TC
L6RFAkEAqvXMF3SM3nB/Nfr9j4eFSszoJgxfzRT/tsM2gU14PfavnNih+6IZld3T
NIkvN6M0xbKASzc+K5F4FifVfONMIQJBAKo4t1757SkcQWj4q3jSLKGgjkFtJyMt
ZPMNuCxSWAAx1wBXX3N70zBTftICB5x+726CAQPRoWCR7NYY+H0Hk80CQQCsZMXJ
lTk9W25zSjWeIJZ1L6m5a5LfT1BWTakjQkgHjptJ7+d085S4Crn4Y/bhJi/00Shy
Zc0jLTXBHUisaCSN
-----END PRIVATE KEY-----
`
)

var _ = g.Describe("[sig-network][Feature:Router][apigroup:route.openshift.io]", func() {
	defer g.GinkgoRecover()
	var (
		oc          *exutil.CLI
		ns          string
		routerImage string
		isFIPS      bool
	)

	g.AfterEach(func() {
		if g.CurrentSpecReport().Failed() {
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
		err = oc.AsAdmin().Run("apply").Args("-f", configPath).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.When("FIPS is enabled", func() {
		g.Describe("the HAProxy router", func() {
			g.It("should not work when configured with a 1024-bit RSA key", g.Label("Size:M"), func() {
				if !isFIPS {
					g.Skip("skipping on non-FIPS cluster")
				}

				routerPod := createScopedRouterPod(routerImage, "test-1024bit", pemData1024, "true")
				g.By("creating a router")
				ns := oc.KubeFramework().Namespace.Name
				_, err := oc.AdminKubeClient().CoreV1().Pods(ns).Create(context.Background(), routerPod, metav1.CreateOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

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
			g.It("should serve routes when configured with a 1024-bit RSA key", g.Label("Size:M"), func() {
				if isFIPS {
					g.Skip("skipping on FIPS cluster")
				}

				routerPod := createScopedRouterPod(routerImage, "test-1024bit", pemData1024, "true")
				g.By("creating a router")
				ns := oc.KubeFramework().Namespace.Name
				_, err := oc.AdminKubeClient().CoreV1().Pods(ns).Create(context.Background(), routerPod, metav1.CreateOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

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
					exurl.Expect("GET", url).Through(exutil.IPUrl(routerIP)).SkipTLSVerification().HasStatusCode(200),
				)
			})
		})
	})
})

func createScopedRouterPod(routerImage, routerName, pemData, updateStatus string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "router-scoped",
			Labels: map[string]string{
				"test": "router-scoped",
			},
		},
		Spec: corev1.PodSpec{
			TerminationGracePeriodSeconds: utilpointer.Int64(1),
			Containers: []corev1.Container{
				{
					Name:            "route",
					Image:           routerImage,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Env: []corev1.EnvVar{
						{
							Name: "POD_NAMESPACE",
							ValueFrom: &corev1.EnvVarSource{
								FieldRef: &corev1.ObjectFieldSelector{
									FieldPath: "metadata.namespace",
								},
							},
						},
						{
							Name:  "ROUTER_IP_V4_V6_MODE",
							Value: "v4v6",
						},
						{
							Name:  "DEFAULT_CERTIFICATE",
							Value: pemData,
						},
					},
					Args: []string{
						"--name=" + routerName,
						"--namespace=$(POD_NAMESPACE)",
						"--update-status=" + updateStatus,
						"-v=4",
						"--labels=select=first",
						"--stats-port=1936",
						"--metrics-type=haproxy",
					},
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: 80,
						},
						{
							ContainerPort: 443,
						},
						{
							ContainerPort: 1936,
							Name:          "stats",
							Protocol:      corev1.ProtocolTCP,
						},
					},
					ReadinessProbe: &corev1.Probe{
						InitialDelaySeconds: 10,
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/healthz/ready",
								Port: intstr.FromInt(1936),
							},
						},
					},
				},
			},
		},
	}
}
