<link rel="stylesheet" type="text/css" href="https://cdnjs.cloudflare.com/ajax/libs/twitter-bootstrap/5.3.0/css/bootstrap.min.css">
<link rel="stylesheet" type="text/css" href="https://cdn.datatables.net/2.0.5/css/dataTables.bootstrap5.css">
<script type="text/javascript" language="javascript" src="https://code.jquery.com/jquery-3.7.1.js"></script>
<script type="text/javascript" language="javascript" src="https://cdn.datatables.net/2.0.5/js/dataTables.js"></script>
<script type="text/javascript" language="javascript" src="https://cdn.datatables.net/2.0.5/js/dataTables.bootstrap5.js"></script> 

<script type="text/javascript">
    $(document).ready(function() {
        new DataTable('#bandwidth-usage');
    });
</script>

<div class="p-3 row">

    <h3>Data Usage</h3>

    <table class="table" id="bandwidth-usage">
        <thead>
            <tr>
                <th scope="col">#</th>
                <th scope="col">SrcIp</th>
                <th scope="col">DestIp</th>
                <th scope="col">Bytes</th>
            </tr>
        </thead>
        <tbody>
            {{ range .BandwidthList }}
            <tr>
                <th scope="row"></th>
                <td>{{ .SrcIpAddr }}</td>
                <td>{{ .DestIpAddr }}</td>
                <td>{{ .Bytes }}</td>
            </tr>
            {{end}}

        </tbody>
    </table>

</div>